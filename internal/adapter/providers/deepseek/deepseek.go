package deepseek

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
	adapter.Register(&DeepSeek{})
}

// DeepSeek adapter discovers models from the DeepSeek API (OpenAI-compatible).
type DeepSeek struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (d *DeepSeek) Name() string { return "deepseek" }

func (d *DeepSeek) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (d *DeepSeek) Configure(apiKey, baseURL string, client *httpclient.Client) {
	d.apiKey = apiKey
	d.baseURL = baseURL
	d.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (d *DeepSeek) HealthCheck(ctx context.Context) error {
	url := d.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + d.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := d.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for DeepSeek.
func (d *DeepSeek) MinExpectedModels() int { return 2 }

func (d *DeepSeek) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := d.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("deepseek API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("deepseek docs source not yet implemented")
		}
	}

	return models, nil
}

// OpenAI-compatible /models response.
type modelsResponse struct {
	Data []apiModel `json:"data"`
}

type apiModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

func (d *DeepSeek) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := d.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + d.apiKey,
	}

	resp, err := d.client.Get(ctx, url, headers)
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

	slog.Info("deepseek API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
	return models, nil
}

func apiModelToDiscovered(am apiModel) *adapter.DiscoveredModel {
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

func inferFamily(id string) string {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "reasoner"):
		return "deepseek-reasoner"
	case strings.Contains(lower, "chat"):
		return "deepseek-chat"
	case strings.Contains(lower, "coder"):
		return "deepseek-coder"
	default:
		return "deepseek"
	}
}

func inferDisplayName(id string) string {
	overrides := map[string]string{
		"deepseek-chat":     "DeepSeek V3",
		"deepseek-reasoner": "DeepSeek R1",
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
	if strings.Contains(lower, "reasoner") {
		caps = append(caps, "reasoning")
	}
	if strings.Contains(lower, "coder") {
		caps = append(caps, "fill_in_middle")
	}
	return caps
}

func inferLimits(id string) adapter.Limits {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "reasoner"):
		return adapter.Limits{MaxTokens: 64000, MaxCompletionTokens: 8192}
	default:
		return adapter.Limits{MaxTokens: 64000, MaxCompletionTokens: 8192}
	}
}
