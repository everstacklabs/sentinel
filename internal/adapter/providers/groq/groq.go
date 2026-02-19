package groq

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
	adapter.Register(&Groq{})
}

// Groq adapter discovers models from the Groq API (OpenAI-compatible).
type Groq struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (g *Groq) Name() string { return "groq" }

func (g *Groq) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (g *Groq) Configure(apiKey, baseURL string, client *httpclient.Client) {
	g.apiKey = apiKey
	g.baseURL = baseURL
	g.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (g *Groq) HealthCheck(ctx context.Context) error {
	url := g.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + g.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := g.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Groq.
func (g *Groq) MinExpectedModels() int { return 5 }

func (g *Groq) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := g.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("groq API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("groq docs source not yet implemented")
		}
	}

	return models, nil
}

// OpenAI-compatible /v1/models response.
type modelsResponse struct {
	Data []apiModel `json:"data"`
}

type apiModel struct {
	ID            string `json:"id"`
	Object        string `json:"object"`
	Created       int64  `json:"created"`
	OwnedBy       string `json:"owned_by"`
	Active        bool   `json:"active"`
	ContextWindow int    `json:"context_window"`
}

func (g *Groq) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := g.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + g.apiKey,
	}

	resp, err := g.client.Get(ctx, url, headers)
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

	slog.Info("groq API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
	return models, nil
}

func apiModelToDiscovered(am apiModel) *adapter.DiscoveredModel {
	if shouldSkip(am) {
		return nil
	}

	contextWindow := am.ContextWindow
	if contextWindow == 0 {
		contextWindow = 8192
	}

	return &adapter.DiscoveredModel{
		Name:         am.ID,
		DisplayName:  inferDisplayName(am.ID),
		Family:       inferFamily(am.ID),
		Status:       "stable",
		Capabilities: inferCapabilities(am.ID),
		Limits:       adapter.Limits{MaxTokens: contextWindow, MaxCompletionTokens: inferMaxCompletion(contextWindow)},
		Modalities:   inferModalities(am.ID),
		DiscoveredBy: adapter.SourceAPI,
	}
}

func shouldSkip(am apiModel) bool {
	lower := strings.ToLower(am.ID)
	// Skip whisper/audio models
	if strings.Contains(lower, "whisper") {
		return true
	}
	// Skip embedding models
	if strings.Contains(lower, "embed") {
		return true
	}
	// Skip inactive models
	if !am.Active && am.ID != "" {
		return false // some APIs don't set Active, default to include
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
	case strings.Contains(lower, "gemma2"):
		return "gemma-2"
	case strings.Contains(lower, "gemma"):
		return "gemma"
	case strings.Contains(lower, "deepseek"):
		return "deepseek"
	case strings.Contains(lower, "qwen"):
		return "qwen"
	default:
		return "groq-other"
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
	if strings.Contains(lower, "vision") || strings.Contains(lower, "llama-3.2-11b") || strings.Contains(lower, "llama-3.2-90b") {
		caps = append(caps, "vision")
	}
	// Most Groq chat models support function calling
	caps = append(caps, "function_calling")
	return caps
}

func inferModalities(id string) adapter.Modalities {
	lower := strings.ToLower(id)
	input := []string{"text"}
	if strings.Contains(lower, "vision") || strings.Contains(lower, "llama-3.2-11b") || strings.Contains(lower, "llama-3.2-90b") {
		input = append(input, "image")
	}
	return adapter.Modalities{
		Input:  input,
		Output: []string{"text"},
	}
}

func inferMaxCompletion(contextWindow int) int {
	if contextWindow >= 128000 {
		return 8192
	}
	if contextWindow >= 32000 {
		return 8192
	}
	return 4096
}
