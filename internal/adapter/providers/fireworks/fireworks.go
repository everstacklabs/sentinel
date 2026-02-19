package fireworks

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
	adapter.Register(&Fireworks{})
}

// Fireworks adapter discovers models from the Fireworks AI API (OpenAI-compatible).
type Fireworks struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (f *Fireworks) Name() string { return "fireworks" }

func (f *Fireworks) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI, adapter.SourceDocs}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (f *Fireworks) Configure(apiKey, baseURL string, client *httpclient.Client) {
	f.apiKey = apiKey
	f.baseURL = baseURL
	f.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (f *Fireworks) HealthCheck(ctx context.Context) error {
	url := f.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + f.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := f.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Fireworks.
func (f *Fireworks) MinExpectedModels() int { return 5 }

func (f *Fireworks) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := f.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("fireworks API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			docModels, err := f.discoverFromDocs(ctx)
			if err != nil {
				slog.Warn("fireworks docs discovery failed, continuing", "error", err)
			} else {
				models = append(models, docModels...)
			}
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

func (f *Fireworks) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
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

	slog.Info("fireworks API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
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
	if strings.Contains(lower, "whisper") {
		return true
	}
	if strings.Contains(lower, "tts") {
		return true
	}
	return false
}

// stripPrefix removes the accounts/fireworks/models/ prefix from model IDs.
func stripPrefix(id string) string {
	return strings.TrimPrefix(id, "accounts/fireworks/models/")
}

func inferFamily(id string) string {
	model := strings.ToLower(stripPrefix(id))
	switch {
	case strings.Contains(model, "llama-v3p3"):
		return "llama-3.3"
	case strings.Contains(model, "llama-v3p2"):
		return "llama-3.2"
	case strings.Contains(model, "llama-v3p1"):
		return "llama-3.1"
	case strings.Contains(model, "llama"):
		return "llama"
	case strings.Contains(model, "mixtral"):
		return "mixtral"
	case strings.Contains(model, "mistral"):
		return "mistral"
	case strings.Contains(model, "qwen"):
		return "qwen"
	case strings.Contains(model, "deepseek"):
		return "deepseek"
	case strings.Contains(model, "gemma"):
		return "gemma"
	default:
		return "fireworks-other"
	}
}

func inferDisplayName(id string) string {
	model := stripPrefix(id)
	parts := strings.Split(model, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func inferCapabilities(id string) []string {
	caps := []string{"chat", "streaming"}
	model := strings.ToLower(stripPrefix(id))
	if strings.Contains(model, "vision") || strings.Contains(model, "11b-vision") {
		caps = append(caps, "vision")
	}
	caps = append(caps, "function_calling")
	return caps
}

func inferLimits(id string) adapter.Limits {
	model := strings.ToLower(stripPrefix(id))
	switch {
	case strings.Contains(model, "llama-v3p1") || strings.Contains(model, "llama-v3p3"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	case strings.Contains(model, "mixtral"):
		return adapter.Limits{MaxTokens: 32768, MaxCompletionTokens: 8192}
	default:
		return adapter.Limits{MaxTokens: 32768, MaxCompletionTokens: 4096}
	}
}

func inferModalities(id string) adapter.Modalities {
	model := strings.ToLower(stripPrefix(id))
	input := []string{"text"}
	if strings.Contains(model, "vision") {
		input = append(input, "image")
	}
	return adapter.Modalities{
		Input:  input,
		Output: []string{"text"},
	}
}
