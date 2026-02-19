package moonshotai

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
	adapter.Register(&MoonshotAI{})
}

// MoonshotAI adapter discovers models from the Moonshot AI (Kimi) API (OpenAI-compatible).
type MoonshotAI struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (m *MoonshotAI) Name() string { return "moonshotai" }

func (m *MoonshotAI) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (m *MoonshotAI) Configure(apiKey, baseURL string, client *httpclient.Client) {
	m.apiKey = apiKey
	m.baseURL = baseURL
	m.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (m *MoonshotAI) HealthCheck(ctx context.Context) error {
	url := m.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + m.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := m.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Moonshot AI.
func (m *MoonshotAI) MinExpectedModels() int { return 2 }

func (m *MoonshotAI) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := m.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("moonshotai API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("moonshotai docs source not yet implemented")
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

func (m *MoonshotAI) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := m.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + m.apiKey,
	}

	resp, err := m.client.Get(ctx, url, headers)
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

	slog.Info("moonshotai API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
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
	case strings.Contains(lower, "kimi"):
		return "kimi"
	case strings.Contains(lower, "moonshot"):
		return "moonshot"
	default:
		return "moonshotai-other"
	}
}

func inferDisplayName(id string) string {
	overrides := map[string]string{
		"moonshot-v1-8k":    "Moonshot V1 8K",
		"moonshot-v1-32k":   "Moonshot V1 32K",
		"moonshot-v1-128k":  "Moonshot V1 128K",
		"kimi-latest":       "Kimi Latest",
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
	return []string{"chat", "function_calling", "streaming"}
}

func inferLimits(id string) adapter.Limits {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "128k"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	case strings.Contains(lower, "32k"):
		return adapter.Limits{MaxTokens: 32768, MaxCompletionTokens: 8192}
	case strings.Contains(lower, "8k"):
		return adapter.Limits{MaxTokens: 8192, MaxCompletionTokens: 4096}
	default:
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	}
}
