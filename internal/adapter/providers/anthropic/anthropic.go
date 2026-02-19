package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/httpclient"
)

func init() {
	adapter.Register(&Anthropic{})
}

// Anthropic adapter discovers models from the Anthropic API.
type Anthropic struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (a *Anthropic) Name() string { return "anthropic" }

func (a *Anthropic) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI, adapter.SourceDocs}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (a *Anthropic) Configure(apiKey, baseURL string, client *httpclient.Client) {
	a.apiKey = apiKey
	a.baseURL = baseURL
	a.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (a *Anthropic) HealthCheck(ctx context.Context) error {
	url := a.baseURL + "/models?limit=1"
	headers := map[string]string{
		"x-api-key":         a.apiKey,
		"anthropic-version": "2023-06-01",
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := a.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Anthropic.
func (a *Anthropic) MinExpectedModels() int { return 4 }

func (a *Anthropic) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := a.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("anthropic API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			docModels, err := a.discoverFromDocs(ctx)
			if err != nil {
				slog.Warn("anthropic docs scraping failed, continuing with API data", "error", err)
			} else {
				models = append(models, docModels...)
			}
		}
	}

	return models, nil
}

// Anthropic /v1/models response types.
type modelsResponse struct {
	Data    []apiModel `json:"data"`
	HasMore bool       `json:"has_more"`
	LastID  string     `json:"last_id"`
}

type apiModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
	Type        string `json:"type"`
}

func (a *Anthropic) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	headers := map[string]string{
		"x-api-key":         a.apiKey,
		"anthropic-version": "2023-06-01",
	}

	var allAPIModels []apiModel
	afterID := ""

	for {
		url := a.baseURL + "/models?limit=1000"
		if afterID != "" {
			url += "&after_id=" + afterID
		}

		resp, err := a.client.Get(ctx, url, headers)
		if err != nil {
			return nil, err
		}

		var modelsResp modelsResponse
		if err := json.Unmarshal(resp.Body, &modelsResp); err != nil {
			return nil, fmt.Errorf("parsing models response: %w", err)
		}

		allAPIModels = append(allAPIModels, modelsResp.Data...)

		if !modelsResp.HasMore {
			break
		}
		afterID = modelsResp.LastID
	}

	var models []adapter.DiscoveredModel
	for _, am := range allAPIModels {
		m := a.apiModelToDiscovered(am)
		if m != nil {
			models = append(models, *m)
		}
	}

	slog.Info("anthropic API discovery complete", "total_api_models", len(allAPIModels), "catalog_models", len(models))
	return models, nil
}

// apiModelToDiscovered converts an Anthropic API model to a DiscoveredModel.
// Returns nil for models we don't want in the catalog (dated snapshots, etc).
func (a *Anthropic) apiModelToDiscovered(am apiModel) *adapter.DiscoveredModel {
	id := am.ID

	if shouldSkip(id) {
		return nil
	}

	family := inferFamily(id)
	displayName := am.DisplayName
	if displayName == "" {
		displayName = inferDisplayName(id)
	}
	capabilities := inferCapabilities(id)
	modalities := inferModalities(id)
	limits := inferLimits(id, family)

	return &adapter.DiscoveredModel{
		Name:         id,
		DisplayName:  displayName,
		Family:       family,
		Status:       "stable",
		Capabilities: capabilities,
		Limits:       adapter.Limits(limits),
		Modalities:   adapter.Modalities(modalities),
		DiscoveredBy: adapter.SourceAPI,
	}
}

// datedSnapshotRe matches dated snapshot IDs like claude-sonnet-4-20250514
// but NOT base aliases like claude-sonnet-4-0 or claude-3-5-sonnet.
var datedSnapshotRe = regexp.MustCompile(`-\d{8}$`)

func shouldSkip(id string) bool {
	// Skip dated snapshots (e.g., claude-sonnet-4-20250514)
	if datedSnapshotRe.MatchString(id) {
		return true
	}
	return false
}

func inferFamily(id string) string {
	switch {
	case strings.Contains(id, "opus"):
		return "claude-opus"
	case strings.Contains(id, "sonnet"):
		return "claude-sonnet"
	case strings.Contains(id, "haiku"):
		return "claude-haiku"
	default:
		return "claude"
	}
}

func inferDisplayName(id string) string {
	// Fallback: capitalize segments
	parts := strings.Split(id, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func inferCapabilities(id string) []string {
	caps := []string{"chat", "function_calling", "vision", "streaming"}

	// All current Claude models support extended_thinking
	caps = append(caps, "extended_thinking")

	// Claude 4.6 models support adaptive_thinking
	if strings.Contains(id, "opus-4-6") || strings.Contains(id, "sonnet-4-6") {
		caps = append(caps, "adaptive_thinking")
	}

	return caps
}

func inferModalities(id string) adapter.Modalities {
	return adapter.Modalities{
		Input:  []string{"text", "image"},
		Output: []string{"text"},
	}
}

func inferLimits(id, family string) adapter.Limits {
	switch {
	case strings.Contains(id, "opus-4-6"):
		return adapter.Limits{MaxTokens: 200000, MaxCompletionTokens: 128000}
	case strings.Contains(id, "sonnet-4-6"):
		return adapter.Limits{MaxTokens: 200000, MaxCompletionTokens: 64000}
	case strings.Contains(id, "sonnet-4-5"), strings.Contains(id, "sonnet-4-0"), strings.Contains(id, "3-7-sonnet"):
		return adapter.Limits{MaxTokens: 200000, MaxCompletionTokens: 64000}
	case strings.Contains(id, "opus-4-5"), strings.Contains(id, "opus-4-1"), strings.Contains(id, "opus-4-0"):
		return adapter.Limits{MaxTokens: 200000, MaxCompletionTokens: 32000}
	case strings.Contains(id, "haiku-4-5"):
		return adapter.Limits{MaxTokens: 200000, MaxCompletionTokens: 64000}
	case family == "claude-haiku":
		return adapter.Limits{MaxTokens: 200000, MaxCompletionTokens: 4096}
	default:
		return adapter.Limits{MaxTokens: 200000, MaxCompletionTokens: 8192}
	}
}
