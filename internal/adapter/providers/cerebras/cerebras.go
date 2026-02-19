package cerebras

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
	adapter.Register(&Cerebras{})
}

// Cerebras adapter discovers models from the Cerebras API (OpenAI-compatible).
type Cerebras struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (c *Cerebras) Name() string { return "cerebras" }

func (c *Cerebras) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (c *Cerebras) Configure(apiKey, baseURL string, client *httpclient.Client) {
	c.apiKey = apiKey
	c.baseURL = baseURL
	c.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (c *Cerebras) HealthCheck(ctx context.Context) error {
	url := c.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + c.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := c.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Cerebras.
func (c *Cerebras) MinExpectedModels() int { return 2 }

func (c *Cerebras) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := c.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("cerebras API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("cerebras docs source not yet implemented")
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

func (c *Cerebras) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := c.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + c.apiKey,
	}

	resp, err := c.client.Get(ctx, url, headers)
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

	slog.Info("cerebras API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
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
		Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		DiscoveredBy: adapter.SourceAPI,
	}
}

func shouldSkip(id string) bool {
	return strings.Contains(strings.ToLower(id), "embed")
}

func inferFamily(id string) string {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "llama"):
		return "llama"
	case strings.Contains(lower, "qwen"):
		return "qwen"
	case strings.Contains(lower, "deepseek"):
		return "deepseek"
	default:
		return "cerebras-other"
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
	if strings.Contains(lower, "vision") {
		caps = append(caps, "vision")
	}
	return caps
}

func inferLimits(id string) adapter.Limits {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "llama-3.3-70b"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	case strings.Contains(lower, "llama-3.1-8b"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	default:
		return adapter.Limits{MaxTokens: 8192, MaxCompletionTokens: 4096}
	}
}
