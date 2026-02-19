package friendli

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
	adapter.Register(&Friendli{})
}

// Friendli adapter discovers models from the Friendli API (OpenAI-compatible).
type Friendli struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (f *Friendli) Name() string { return "friendli" }

func (f *Friendli) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (f *Friendli) Configure(apiKey, baseURL string, client *httpclient.Client) {
	f.apiKey = apiKey
	f.baseURL = baseURL
	f.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (f *Friendli) HealthCheck(ctx context.Context) error {
	url := f.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + f.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := f.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Friendli.
func (f *Friendli) MinExpectedModels() int { return 3 }

func (f *Friendli) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := f.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("friendli API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("friendli docs source not yet implemented")
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
	OwnedBy string `json:"owned_by"`
}

func (f *Friendli) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := f.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + f.apiKey,
	}

	resp, err := f.client.Get(ctx, url, headers)
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

	slog.Info("friendli API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
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
	if strings.Contains(lower, "embed") {
		return true
	}
	if strings.Contains(lower, "rerank") {
		return true
	}
	return false
}

func inferFamily(id string) string {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "llama-3.3"):
		return "llama-3.3"
	case strings.Contains(lower, "llama-3.2"):
		return "llama-3.2"
	case strings.Contains(lower, "llama-3.1"):
		return "llama-3.1"
	case strings.Contains(lower, "llama"):
		return "llama"
	case strings.Contains(lower, "mixtral"):
		return "mixtral"
	case strings.Contains(lower, "mistral"):
		return "mistral"
	case strings.Contains(lower, "qwen"):
		return "qwen"
	case strings.Contains(lower, "deepseek"):
		return "deepseek"
	case strings.Contains(lower, "gemma"):
		return "gemma"
	default:
		return "friendli-other"
	}
}

func inferDisplayName(id string) string {
	parts := strings.Split(id, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func inferCapabilities(id string) []string {
	caps := []string{"chat", "streaming"}
	lower := strings.ToLower(id)
	if strings.Contains(lower, "vision") || strings.Contains(lower, "vl") {
		caps = append(caps, "vision")
	}
	caps = append(caps, "function_calling")
	return caps
}

func inferLimits(id string) adapter.Limits {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "llama-3.1") || strings.Contains(lower, "llama-3.3"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	default:
		return adapter.Limits{MaxTokens: 32768, MaxCompletionTokens: 4096}
	}
}

func inferModalities(id string) adapter.Modalities {
	lower := strings.ToLower(id)
	input := []string{"text"}
	if strings.Contains(lower, "vision") || strings.Contains(lower, "vl") {
		input = append(input, "image")
	}
	return adapter.Modalities{
		Input:  input,
		Output: []string{"text"},
	}
}
