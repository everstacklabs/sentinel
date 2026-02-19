package togetherai

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
	adapter.Register(&TogetherAI{})
}

// TogetherAI adapter discovers models from the Together AI API.
type TogetherAI struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (t *TogetherAI) Name() string { return "togetherai" }

func (t *TogetherAI) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI, adapter.SourceDocs}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (t *TogetherAI) Configure(apiKey, baseURL string, client *httpclient.Client) {
	t.apiKey = apiKey
	t.baseURL = baseURL
	t.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (t *TogetherAI) HealthCheck(ctx context.Context) error {
	url := t.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + t.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := t.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Together AI.
func (t *TogetherAI) MinExpectedModels() int { return 20 }

func (t *TogetherAI) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := t.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("togetherai API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			docModels, err := t.discoverFromDocs(ctx)
			if err != nil {
				slog.Warn("togetherai docs discovery failed, continuing", "error", err)
			} else {
				models = append(models, docModels...)
			}
		}
	}

	return models, nil
}

// Together AI /v1/models response — returns a flat array.
type apiModel struct {
	ID            string `json:"id"`
	Object        string `json:"object"`
	Created       int64  `json:"created"`
	Type          string `json:"type"`
	DisplayName   string `json:"display_name"`
	Organization  string `json:"organization"`
	ContextLength int    `json:"context_length"`
	Pricing       *struct {
		Input    float64 `json:"input"`
		Output   float64 `json:"output"`
	} `json:"pricing"`
}

func (t *TogetherAI) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := t.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + t.apiKey,
	}

	resp, err := t.client.Get(ctx, url, headers)
	if err != nil {
		return nil, err
	}

	var allModels []apiModel
	if err := json.Unmarshal(resp.Body, &allModels); err != nil {
		return nil, fmt.Errorf("parsing models response: %w", err)
	}

	var models []adapter.DiscoveredModel
	for _, am := range allModels {
		m := apiModelToDiscovered(am)
		if m != nil {
			models = append(models, *m)
		}
	}

	slog.Info("togetherai API discovery complete", "total_api_models", len(allModels), "catalog_models", len(models))
	return models, nil
}

func apiModelToDiscovered(am apiModel) *adapter.DiscoveredModel {
	if shouldSkip(am) {
		return nil
	}

	displayName := am.DisplayName
	if displayName == "" {
		displayName = inferDisplayName(am.ID)
	}

	contextLength := am.ContextLength
	if contextLength == 0 {
		contextLength = 4096
	}

	m := &adapter.DiscoveredModel{
		Name:         am.ID,
		DisplayName:  displayName,
		Family:       inferFamily(am.ID),
		Status:       "stable",
		Capabilities: inferCapabilities(am),
		Limits:       adapter.Limits{MaxTokens: contextLength, MaxCompletionTokens: inferMaxCompletion(contextLength)},
		Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		DiscoveredBy: adapter.SourceAPI,
	}

	// Together AI returns pricing per token; convert to per 1K.
	if am.Pricing != nil && (am.Pricing.Input > 0 || am.Pricing.Output > 0) {
		m.Cost = &adapter.Cost{
			InputPer1K:  am.Pricing.Input * 1000,
			OutputPer1K: am.Pricing.Output * 1000,
		}
	}

	return m
}

func shouldSkip(am apiModel) bool {
	// Only include chat/language models
	switch am.Type {
	case "chat", "language", "code":
		return false
	case "image", "embedding", "moderation", "rerank", "audio":
		return true
	}
	// If type is empty, check the ID for hints
	lower := strings.ToLower(am.ID)
	if strings.Contains(lower, "embed") || strings.Contains(lower, "rerank") {
		return true
	}
	return false
}

func inferFamily(id string) string {
	lower := strings.ToLower(id)
	// Together AI uses org/model format
	parts := strings.Split(lower, "/")
	modelPart := lower
	if len(parts) > 1 {
		modelPart = parts[len(parts)-1]
	}

	switch {
	case strings.Contains(modelPart, "llama-3.3"):
		return "llama-3.3"
	case strings.Contains(modelPart, "llama-3.2"):
		return "llama-3.2"
	case strings.Contains(modelPart, "llama-3.1"):
		return "llama-3.1"
	case strings.Contains(modelPart, "llama-3"):
		return "llama-3"
	case strings.Contains(modelPart, "llama"):
		return "llama"
	case strings.Contains(modelPart, "mixtral"):
		return "mixtral"
	case strings.Contains(modelPart, "mistral"):
		return "mistral"
	case strings.Contains(modelPart, "qwen"):
		return "qwen"
	case strings.Contains(modelPart, "deepseek"):
		return "deepseek"
	case strings.Contains(modelPart, "gemma"):
		return "gemma"
	case strings.Contains(modelPart, "yi"):
		return "yi"
	case strings.Contains(modelPart, "dbrx"):
		return "dbrx"
	default:
		return "togetherai-other"
	}
}

func inferDisplayName(id string) string {
	// Strip org prefix for display
	parts := strings.Split(id, "/")
	name := parts[len(parts)-1]
	segments := strings.Split(name, "-")
	for i, p := range segments {
		if len(p) > 0 {
			segments[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(segments, " ")
}

func inferCapabilities(am apiModel) []string {
	caps := []string{"chat", "streaming"}
	lower := strings.ToLower(am.ID)
	if am.Type == "code" || strings.Contains(lower, "code") {
		caps = append(caps, "fill_in_middle")
	}
	// Most larger chat models support function calling
	if am.ContextLength >= 8192 {
		caps = append(caps, "function_calling")
	}
	return caps
}

func inferMaxCompletion(contextLength int) int {
	if contextLength >= 128000 {
		return 8192
	}
	if contextLength >= 32000 {
		return 8192
	}
	return 4096
}
