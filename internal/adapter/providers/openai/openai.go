package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/httpclient"
)

func init() {
	adapter.Register(&OpenAI{})
}

// OpenAI adapter discovers models from the OpenAI API.
type OpenAI struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (o *OpenAI) Name() string { return "openai" }

func (o *OpenAI) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (o *OpenAI) Configure(apiKey, baseURL string, client *httpclient.Client) {
	o.apiKey = apiKey
	o.baseURL = baseURL
	o.client = client
}

func (o *OpenAI) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := o.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("openai API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Warn("openai docs source not implemented in Phase 1")
		}
	}

	return models, nil
}

// OpenAI /v1/models response types.
type modelsResponse struct {
	Data []apiModel `json:"data"`
}

type apiModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func (o *OpenAI) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := o.baseURL + "/models"

	headers := map[string]string{
		"Authorization": "Bearer " + o.apiKey,
	}

	resp, err := o.client.Get(ctx, url, headers)
	if err != nil {
		return nil, err
	}

	var modelsResp modelsResponse
	if err := json.Unmarshal(resp.Body, &modelsResp); err != nil {
		return nil, fmt.Errorf("parsing models response: %w", err)
	}

	var models []adapter.DiscoveredModel
	for _, am := range modelsResp.Data {
		m := o.apiModelToDiscovered(am)
		if m != nil {
			models = append(models, *m)
		}
	}

	slog.Info("openai API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
	return models, nil
}

// apiModelToDiscovered converts an OpenAI API model to a DiscoveredModel.
// Returns nil for models we don't want in the catalog (system models, snapshots, etc).
func (o *OpenAI) apiModelToDiscovered(am apiModel) *adapter.DiscoveredModel {
	id := am.ID

	// Skip system/internal models
	if o.shouldSkip(id) {
		return nil
	}

	family := inferFamily(id)
	displayName := inferDisplayName(id)
	capabilities := inferCapabilities(id)
	modalities := inferModalities(id, capabilities)
	limits := inferLimits(id, family)

	return &adapter.DiscoveredModel{
		Name:         id,
		DisplayName:  displayName,
		Family:       family,
		Status:       "stable",
		Capabilities: capabilities,
		Limits:       adapter.Limits(limits),
		Modalities:   adapter.Modalities(modalities),
		DiscoveredBy: adapter.SourceAPI,
	}
}

func (o *OpenAI) shouldSkip(id string) bool {
	// Skip fine-tuned models
	if strings.HasPrefix(id, "ft:") {
		return true
	}
	// Skip dated snapshots (e.g., gpt-4-0613) — keep only the base alias
	if isDateSnapshot(id) {
		return true
	}
	// Skip internal/system models
	skipPrefixes := []string{"dall-e", "tts-", "whisper", "text-moderation", "babbage", "davinci", "curie", "ada-"}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}

func isDateSnapshot(id string) bool {
	// Pattern: any segment that looks like a date (MMDD or YYYYMMDD)
	// e.g., gpt-4-0613, gpt-4-1106-preview, gpt-4o-2024-05-13, gpt-5-2025-08-07
	parts := strings.Split(id, "-")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts[1:] { // skip the first segment (model prefix)
		if isDateLike(p) {
			return true
		}
	}
	// Also check for YYYY-MM-DD pattern: three consecutive numeric segments
	for i := 1; i+2 < len(parts); i++ {
		if len(parts[i]) == 4 && len(parts[i+1]) == 2 && len(parts[i+2]) == 2 &&
			isAllDigits(parts[i]) && isAllDigits(parts[i+1]) && isAllDigits(parts[i+2]) {
			return true
		}
	}
	return false
}

func isDateLike(s string) bool {
	if len(s) != 4 && len(s) != 8 {
		return false
	}
	return isAllDigits(s)
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func inferFamily(id string) string {
	switch {
	case strings.HasPrefix(id, "gpt-5.3"):
		return "gpt-5"
	case strings.HasPrefix(id, "gpt-5.2"):
		return "gpt-5"
	case strings.HasPrefix(id, "gpt-5.1"):
		return "gpt-5"
	case strings.HasPrefix(id, "gpt-5"):
		return "gpt-5"
	case strings.HasPrefix(id, "gpt-4o"):
		return "gpt-4"
	case strings.HasPrefix(id, "gpt-4.1"):
		return "gpt-4"
	case strings.HasPrefix(id, "gpt-4-turbo"):
		return "gpt-4"
	case strings.HasPrefix(id, "gpt-4"):
		return "gpt-4"
	case strings.HasPrefix(id, "gpt-3.5"):
		return "gpt-3.5"
	case strings.HasPrefix(id, "o4"):
		return "o-series"
	case strings.HasPrefix(id, "o3"):
		return "o-series"
	case strings.HasPrefix(id, "o1"):
		return "o-series"
	case strings.HasPrefix(id, "text-embedding"):
		return "embedding"
	default:
		return "other"
	}
}

func inferDisplayName(id string) string {
	// Known display name overrides for common models
	overrides := map[string]string{
		"gpt-4o":                  "GPT-4o",
		"gpt-4o-mini":             "GPT-4o Mini",
		"gpt-4-turbo":             "GPT-4 Turbo",
		"gpt-4":                   "GPT-4",
		"gpt-3.5-turbo":           "GPT-3.5 Turbo",
		"gpt-3.5-turbo-instruct":  "GPT-3.5 Turbo Instruct",
		"gpt-3.5-turbo-16k":       "GPT-3.5 Turbo 16K",
		"gpt-5":                   "GPT-5",
		"gpt-5-mini":              "GPT-5 Mini",
		"gpt-5-nano":              "GPT-5 Nano",
		"gpt-5-pro":               "GPT-5 Pro",
		"gpt-5-codex":             "GPT-5 Codex",
		"gpt-5.1":                 "GPT-5.1",
		"gpt-5.1-codex":           "GPT-5.1 Codex",
		"gpt-5.1-codex-mini":      "GPT-5.1 Codex Mini",
		"gpt-5.1-codex-max":       "GPT-5.1 Codex Max",
		"gpt-5.2":                 "GPT-5.2",
		"gpt-5.2-codex":           "GPT-5.2 Codex",
		"gpt-5.2-pro":             "GPT-5.2 Pro",
		"gpt-5.3-codex":           "GPT-5.3 Codex",
		"gpt-5.3-codex-spark":     "GPT-5.3 Codex Spark",
		"gpt-4.1":                 "GPT-4.1",
		"gpt-4.1-mini":            "GPT-4.1 Mini",
		"gpt-4.1-nano":            "GPT-4.1 Nano",
		"o1":                      "O1",
		"o1-mini":                 "O1 Mini",
		"o1-pro":                  "O1 Pro",
		"o3":                      "O3",
		"o3-mini":                 "O3 Mini",
		"o4-mini":                 "O4 Mini",
		"text-embedding-3-small":  "Text Embedding 3 Small",
		"text-embedding-3-large":  "Text Embedding 3 Large",
		"text-embedding-ada-002":  "Text Embedding Ada 002",
	}

	if name, ok := overrides[id]; ok {
		return name
	}

	// Fallback: capitalize segments
	parts := strings.Split(id, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func inferCapabilities(id string) []string {
	caps := []string{"chat"}

	// Embeddings models
	if strings.Contains(id, "embedding") {
		return []string{"embeddings"}
	}

	// Function calling
	if !strings.Contains(id, "instruct") {
		caps = append(caps, "function_calling")
	}

	// Vision
	if strings.Contains(id, "gpt-4o") || strings.Contains(id, "gpt-4-turbo") ||
		strings.HasPrefix(id, "gpt-5") || strings.HasPrefix(id, "gpt-4.1") {
		caps = append(caps, "vision")
	}

	return caps
}

func inferModalities(id string, capabilities []string) adapter.Modalities {
	for _, c := range capabilities {
		if c == "embeddings" {
			return adapter.Modalities{
				Input:  []string{"text"},
				Output: []string{"embedding"},
			}
		}
	}

	input := []string{"text"}
	for _, c := range capabilities {
		if c == "vision" {
			input = append(input, "image")
			break
		}
	}

	return adapter.Modalities{
		Input:  input,
		Output: []string{"text"},
	}
}

func inferLimits(id, family string) adapter.Limits {
	switch family {
	case "gpt-5":
		return adapter.Limits{MaxTokens: 128000, MaxCompletionTokens: 16384}
	case "gpt-4":
		if strings.Contains(id, "mini") {
			return adapter.Limits{MaxTokens: 128000, MaxCompletionTokens: 16384}
		}
		return adapter.Limits{MaxTokens: 128000, MaxCompletionTokens: 16384}
	case "gpt-3.5":
		return adapter.Limits{MaxTokens: 16385, MaxCompletionTokens: 4096}
	case "o-series":
		return adapter.Limits{MaxTokens: 200000, MaxCompletionTokens: 100000}
	case "embedding":
		return adapter.Limits{MaxTokens: 8191}
	default:
		return adapter.Limits{MaxTokens: 128000}
	}
}
