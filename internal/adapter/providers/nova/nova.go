package nova

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
	adapter.Register(&Nova{})
}

// Nova adapter discovers models from the Amazon Nova API (OpenAI-compatible).
type Nova struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (n *Nova) Name() string { return "nova" }

func (n *Nova) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (n *Nova) Configure(apiKey, baseURL string, client *httpclient.Client) {
	n.apiKey = apiKey
	n.baseURL = baseURL
	n.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (n *Nova) HealthCheck(ctx context.Context) error {
	url := n.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + n.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := n.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Amazon Nova.
func (n *Nova) MinExpectedModels() int { return 1 }

func (n *Nova) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := n.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("nova API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("nova docs source not yet implemented")
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

func (n *Nova) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := n.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + n.apiKey,
	}

	resp, err := n.client.Get(ctx, url, headers)
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

	slog.Info("nova API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
	return models, nil
}

func apiModelToDiscovered(am apiModel) *adapter.DiscoveredModel {
	if shouldSkip(am.ID) {
		return nil
	}

	return &adapter.DiscoveredModel{
		Name:         am.ID,
		DisplayName:  inferDisplayName(am.ID),
		Family:       "nova",
		Status:       "stable",
		Capabilities: []string{"chat", "function_calling", "streaming"},
		Limits:       adapter.Limits{MaxTokens: 300000, MaxCompletionTokens: 5120},
		Modalities:   inferModalities(am.ID),
		DiscoveredBy: adapter.SourceAPI,
	}
}

func shouldSkip(id string) bool {
	lower := strings.ToLower(id)
	if strings.Contains(lower, "embed") {
		return true
	}
	if strings.Contains(lower, "canvas") || strings.Contains(lower, "reel") {
		return true
	}
	return false
}

func inferDisplayName(id string) string {
	overrides := map[string]string{
		"us.amazon.nova-pro-v1:0":   "Amazon Nova Pro",
		"us.amazon.nova-lite-v1:0":  "Amazon Nova Lite",
		"us.amazon.nova-micro-v1:0": "Amazon Nova Micro",
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

func inferModalities(id string) adapter.Modalities {
	lower := strings.ToLower(id)
	input := []string{"text"}
	if strings.Contains(lower, "pro") || strings.Contains(lower, "lite") {
		input = append(input, "image")
	}
	return adapter.Modalities{
		Input:  input,
		Output: []string{"text"},
	}
}
