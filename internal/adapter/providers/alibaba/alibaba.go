package alibaba

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
	adapter.Register(&Alibaba{})
}

// Alibaba adapter discovers models from the Alibaba/DashScope API (OpenAI-compatible).
type Alibaba struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (a *Alibaba) Name() string { return "alibaba" }

func (a *Alibaba) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (a *Alibaba) Configure(apiKey, baseURL string, client *httpclient.Client) {
	a.apiKey = apiKey
	a.baseURL = baseURL
	a.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (a *Alibaba) HealthCheck(ctx context.Context) error {
	url := a.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + a.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := a.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Alibaba.
func (a *Alibaba) MinExpectedModels() int { return 5 }

func (a *Alibaba) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := a.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("alibaba API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("alibaba docs source not yet implemented")
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

func (a *Alibaba) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := a.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + a.apiKey,
	}

	resp, err := a.client.Get(ctx, url, headers)
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

	slog.Info("alibaba API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
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
	if strings.Contains(lower, "audio") || strings.Contains(lower, "paraformer") {
		return true
	}
	if strings.Contains(lower, "image") || strings.Contains(lower, "wanx") {
		return true
	}
	return false
}

func inferFamily(id string) string {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "qwen3"):
		return "qwen-3"
	case strings.Contains(lower, "qwen2.5"):
		return "qwen-2.5"
	case strings.Contains(lower, "qwen2"):
		return "qwen-2"
	case strings.Contains(lower, "qwen-turbo"):
		return "qwen-turbo"
	case strings.Contains(lower, "qwen-plus"):
		return "qwen-plus"
	case strings.Contains(lower, "qwen-max"):
		return "qwen-max"
	case strings.Contains(lower, "qwen-long"):
		return "qwen-long"
	case strings.Contains(lower, "qwen"):
		return "qwen"
	default:
		return "alibaba-other"
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
	caps := []string{"chat", "function_calling", "streaming"}
	lower := strings.ToLower(id)
	if strings.Contains(lower, "vl") || strings.Contains(lower, "vision") {
		caps = append(caps, "vision")
	}
	if strings.Contains(lower, "coder") || strings.Contains(lower, "code") {
		caps = append(caps, "fill_in_middle")
	}
	return caps
}

func inferLimits(id string) adapter.Limits {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "qwen-long"):
		return adapter.Limits{MaxTokens: 1000000, MaxCompletionTokens: 8192}
	case strings.Contains(lower, "qwen-max"):
		return adapter.Limits{MaxTokens: 32768, MaxCompletionTokens: 8192}
	case strings.Contains(lower, "qwen-plus"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	case strings.Contains(lower, "qwen-turbo"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	default:
		return adapter.Limits{MaxTokens: 32768, MaxCompletionTokens: 8192}
	}
}

func inferModalities(id string) adapter.Modalities {
	lower := strings.ToLower(id)
	input := []string{"text"}
	if strings.Contains(lower, "vl") || strings.Contains(lower, "vision") {
		input = append(input, "image")
	}
	return adapter.Modalities{
		Input:  input,
		Output: []string{"text"},
	}
}
