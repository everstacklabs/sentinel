package xai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/httpclient"
)

func init() {
	adapter.Register(&XAI{})
}

// XAI adapter discovers models from the xAI (Grok) API.
type XAI struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (x *XAI) Name() string { return "xai" }

func (x *XAI) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (x *XAI) Configure(apiKey, baseURL string, client *httpclient.Client) {
	x.apiKey = apiKey
	x.baseURL = baseURL
	x.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (x *XAI) HealthCheck(ctx context.Context) error {
	url := x.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + x.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := x.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for xAI.
func (x *XAI) MinExpectedModels() int { return 3 }

func (x *XAI) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := x.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("xai API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("xai docs source not yet implemented")
		}
	}

	return models, nil
}

// OpenAI-compatible /v1/models response.
type modelsResponse struct {
	Data []apiModel `json:"data"`
}

type apiModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func (x *XAI) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := x.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + x.apiKey,
	}

	resp, err := x.client.Get(ctx, url, headers)
	if err != nil {
		return nil, err
	}

	var modelsResp modelsResponse
	if err := json.Unmarshal(resp.Body, &modelsResp); err != nil {
		return nil, fmt.Errorf("parsing models response: %w", err)
	}

	var models []adapter.DiscoveredModel
	for _, am := range modelsResp.Data {
		m := apiModelToDiscovered(am)
		if m != nil {
			models = append(models, *m)
		}
	}

	slog.Info("xai API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
	return models, nil
}

func apiModelToDiscovered(am apiModel) *adapter.DiscoveredModel {
	if shouldSkip(am.ID) {
		return nil
	}

	return &adapter.DiscoveredModel{
		Name:         am.ID,
		DisplayName:  inferDisplayName(am.ID),
		Family:       inferFamily(am.ID),
		Status:       "stable",
		Capabilities: inferCapabilities(am.ID),
		Limits:       inferLimits(am.ID),
		Modalities:   inferModalities(am.ID),
		DiscoveredBy: adapter.SourceAPI,
	}
}

func shouldSkip(id string) bool {
	lower := strings.ToLower(id)
	// Skip image generation models
	if strings.Contains(lower, "image") {
		return true
	}
	// Skip embedding models
	if strings.Contains(lower, "embed") {
		return true
	}
	return false
}

func inferFamily(id string) string {
	lower := strings.ToLower(id)
	switch {
	case strings.HasPrefix(lower, "grok-4"):
		return "grok-4"
	case strings.HasPrefix(lower, "grok-3"):
		return "grok-3"
	case strings.HasPrefix(lower, "grok-2"):
		return "grok-2"
	case strings.HasPrefix(lower, "grok"):
		return "grok"
	default:
		return "xai-other"
	}
}

func inferDisplayName(id string) string {
	overrides := map[string]string{
		"grok-4":        "Grok 4",
		"grok-4-fast":   "Grok 4 Fast",
		"grok-3":        "Grok 3",
		"grok-3-fast":   "Grok 3 Fast",
		"grok-3-mini":   "Grok 3 Mini",
		"grok-3-mini-fast": "Grok 3 Mini Fast",
		"grok-2":        "Grok 2",
		"grok-2-vision": "Grok 2 Vision",
	}
	if name, ok := overrides[id]; ok {
		return name
	}
	parts := strings.Split(id, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func inferCapabilities(id string) []string {
	lower := strings.ToLower(id)
	caps := []string{"chat", "function_calling", "streaming"}
	if strings.Contains(lower, "vision") {
		caps = append(caps, "vision")
	}
	if strings.Contains(lower, "mini") {
		caps = append(caps, "reasoning")
	}
	return caps
}

func inferLimits(id string) adapter.Limits {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "grok-4"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 16384}
	case strings.Contains(lower, "grok-3"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 16384}
	case strings.Contains(lower, "grok-2"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	default:
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	}
}

func inferModalities(id string) adapter.Modalities {
	lower := strings.ToLower(id)
	input := []string{"text"}
	if strings.Contains(lower, "vision") {
		input = append(input, "image")
	}
	return adapter.Modalities{
		Input:  input,
		Output: []string{"text"},
	}
}
