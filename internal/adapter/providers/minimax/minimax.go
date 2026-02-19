package minimax

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
	adapter.Register(&MiniMax{})
}

// MiniMax adapter discovers models from the MiniMax API (OpenAI-compatible).
type MiniMax struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (m *MiniMax) Name() string { return "minimax" }

func (m *MiniMax) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (m *MiniMax) Configure(apiKey, baseURL string, client *httpclient.Client) {
	m.apiKey = apiKey
	m.baseURL = baseURL
	m.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (m *MiniMax) HealthCheck(ctx context.Context) error {
	url := m.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + m.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := m.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for MiniMax.
func (m *MiniMax) MinExpectedModels() int { return 2 }

func (m *MiniMax) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := m.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("minimax API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("minimax docs source not yet implemented")
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

func (m *MiniMax) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
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

	slog.Info("minimax API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
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
	lower := strings.ToLower(id)
	if strings.Contains(lower, "embed") {
		return true
	}
	if strings.Contains(lower, "speech") || strings.Contains(lower, "tts") {
		return true
	}
	return false
}

func inferFamily(id string) string {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "m2"):
		return "minimax-m2"
	case strings.Contains(lower, "minimax"):
		return "minimax"
	default:
		return "minimax-other"
	}
}

func inferDisplayName(id string) string {
	overrides := map[string]string{
		"MiniMax-M1":   "MiniMax M1",
		"MiniMax-M1-8": "MiniMax M1 8",
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
	return adapter.Limits{MaxTokens: 1000000, MaxCompletionTokens: 8192}
}
