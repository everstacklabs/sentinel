package zhipuai

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
	adapter.Register(&ZhipuAI{})
}

// ZhipuAI adapter discovers models from the Zhipu AI API.
// Uses /v4 API path (not standard /v1).
type ZhipuAI struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (z *ZhipuAI) Name() string { return "zhipuai" }

func (z *ZhipuAI) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (z *ZhipuAI) Configure(apiKey, baseURL string, client *httpclient.Client) {
	z.apiKey = apiKey
	z.baseURL = baseURL
	z.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (z *ZhipuAI) HealthCheck(ctx context.Context) error {
	url := z.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + z.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := z.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Zhipu AI.
func (z *ZhipuAI) MinExpectedModels() int { return 3 }

func (z *ZhipuAI) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := z.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("zhipuai API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("zhipuai docs source not yet implemented")
		}
	}

	return models, nil
}

// OpenAI-compatible models response.
type modelsResponse struct {
	Data []apiModel `json:"data"`
}

type apiModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

func (z *ZhipuAI) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := z.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + z.apiKey,
	}

	resp, err := z.client.Get(ctx, url, headers)
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

	slog.Info("zhipuai API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
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
	if strings.Contains(lower, "cogview") || strings.Contains(lower, "cogvideo") {
		return true
	}
	return false
}

func inferFamily(id string) string {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "glm-4"):
		return "glm-4"
	case strings.Contains(lower, "glm-3"):
		return "glm-3"
	case strings.Contains(lower, "glm"):
		return "glm"
	default:
		return "zhipuai-other"
	}
}

func inferDisplayName(id string) string {
	overrides := map[string]string{
		"glm-4":       "GLM 4",
		"glm-4-plus":  "GLM 4 Plus",
		"glm-4-air":   "GLM 4 Air",
		"glm-4-airx":  "GLM 4 AirX",
		"glm-4-flash": "GLM 4 Flash",
		"glm-4-long":  "GLM 4 Long",
		"glm-4v":      "GLM 4V",
		"glm-4v-plus": "GLM 4V Plus",
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
	caps := []string{"chat", "function_calling", "streaming"}
	lower := strings.ToLower(id)
	if strings.Contains(lower, "4v") || strings.Contains(lower, "vision") {
		caps = append(caps, "vision")
	}
	return caps
}

func inferLimits(id string) adapter.Limits {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "long"):
		return adapter.Limits{MaxTokens: 1000000, MaxCompletionTokens: 8192}
	case strings.Contains(lower, "glm-4"):
		return adapter.Limits{MaxTokens: 128000, MaxCompletionTokens: 8192}
	default:
		return adapter.Limits{MaxTokens: 8192, MaxCompletionTokens: 4096}
	}
}

func inferModalities(id string) adapter.Modalities {
	lower := strings.ToLower(id)
	input := []string{"text"}
	if strings.Contains(lower, "4v") || strings.Contains(lower, "vision") {
		input = append(input, "image")
	}
	return adapter.Modalities{
		Input:  input,
		Output: []string{"text"},
	}
}
