package llama

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
	adapter.Register(&Llama{})
}

// Llama adapter discovers models from the Meta Llama API (OpenAI-compatible).
type Llama struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (l *Llama) Name() string { return "llama" }

func (l *Llama) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (l *Llama) Configure(apiKey, baseURL string, client *httpclient.Client) {
	l.apiKey = apiKey
	l.baseURL = baseURL
	l.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (l *Llama) HealthCheck(ctx context.Context) error {
	url := l.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + l.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := l.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Meta Llama.
func (l *Llama) MinExpectedModels() int { return 3 }

func (l *Llama) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := l.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("llama API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("llama docs source not yet implemented")
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

func (l *Llama) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := l.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + l.apiKey,
	}

	resp, err := l.client.Get(ctx, url, headers)
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

	slog.Info("llama API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
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
	if strings.Contains(lower, "guard") {
		return true
	}
	if strings.Contains(lower, "embed") {
		return true
	}
	return false
}

func inferFamily(id string) string {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "llama-4"):
		return "llama-4"
	case strings.Contains(lower, "llama-3.3"):
		return "llama-3.3"
	case strings.Contains(lower, "llama-3.2"):
		return "llama-3.2"
	case strings.Contains(lower, "llama-3.1"):
		return "llama-3.1"
	default:
		return "llama"
	}
}

func inferDisplayName(id string) string {
	overrides := map[string]string{
		"Llama-4-Maverick-17B-128E": "Llama 4 Maverick 17B 128E",
		"Llama-4-Scout-17B-16E":     "Llama 4 Scout 17B 16E",
		"Llama-3.3-70B-Instruct":    "Llama 3.3 70B Instruct",
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
	if strings.Contains(lower, "vision") || strings.Contains(lower, "scout") || strings.Contains(lower, "maverick") {
		caps = append(caps, "vision")
	}
	return caps
}

func inferLimits(id string) adapter.Limits {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "llama-4"):
		return adapter.Limits{MaxTokens: 1048576, MaxCompletionTokens: 16384}
	case strings.Contains(lower, "llama-3.3") || strings.Contains(lower, "llama-3.1"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	default:
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	}
}

func inferModalities(id string) adapter.Modalities {
	lower := strings.ToLower(id)
	input := []string{"text"}
	if strings.Contains(lower, "vision") || strings.Contains(lower, "scout") || strings.Contains(lower, "maverick") {
		input = append(input, "image")
	}
	return adapter.Modalities{
		Input:  input,
		Output: []string{"text"},
	}
}
