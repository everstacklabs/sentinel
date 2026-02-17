package mistral

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
	adapter.Register(&Mistral{})
}

// Mistral adapter discovers models from the Mistral AI API.
type Mistral struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (m *Mistral) Name() string { return "mistral" }

func (m *Mistral) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (m *Mistral) Configure(apiKey, baseURL string, client *httpclient.Client) {
	m.apiKey = apiKey
	m.baseURL = baseURL
	m.client = client
}

func (m *Mistral) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := m.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("mistral API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Warn("mistral docs source not implemented in Phase 1")
		}
	}

	return models, nil
}

// Mistral /v1/models response types.
type modelsResponse struct {
	Data []apiModel `json:"data"`
}

type apiModelCapabilities struct {
	CompletionChat bool `json:"completion_chat"`
	CompletionFIM  bool `json:"completion_fim"`
	FunctionCalling bool `json:"function_calling"`
	FineTuning     bool `json:"fine_tuning"`
	Vision         bool `json:"vision"`
}

type apiModel struct {
	ID                string               `json:"id"`
	Object            string               `json:"object"`
	Created           int64                `json:"created"`
	OwnedBy           string               `json:"owned_by"`
	Name              string               `json:"name"`
	Description       string               `json:"description"`
	MaxContextLength  int                  `json:"max_context_length"`
	Aliases           []string             `json:"aliases"`
	Capabilities      apiModelCapabilities `json:"capabilities"`
	Type              string               `json:"type"`
	Deprecation       *string              `json:"deprecation"`
}

func (m *Mistral) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
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
		dm := m.apiModelToDiscovered(am)
		if dm != nil {
			models = append(models, *dm)
		}
	}

	slog.Info("mistral API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
	return models, nil
}

func (m *Mistral) apiModelToDiscovered(am apiModel) *adapter.DiscoveredModel {
	if shouldSkip(am) {
		return nil
	}

	family := inferFamily(am.ID)
	displayName := am.Name
	if displayName == "" {
		displayName = inferDisplayName(am.ID)
	}

	capabilities := buildCapabilities(am.Capabilities)
	modalities := inferModalities(am.Capabilities)

	status := "stable"
	if am.Deprecation != nil {
		status = "deprecated"
	}

	return &adapter.DiscoveredModel{
		Name:         am.ID,
		DisplayName:  displayName,
		Family:       family,
		Status:       status,
		Capabilities: capabilities,
		Limits:       adapter.Limits{MaxTokens: am.MaxContextLength, MaxCompletionTokens: inferMaxCompletion(am.ID, am.MaxContextLength)},
		Modalities:   modalities,
		DiscoveredBy: adapter.SourceAPI,
	}
}

func shouldSkip(am apiModel) bool {
	// Skip fine-tuned models
	if am.Type == "fine-tuned" {
		return true
	}
	// Skip deprecated models
	if am.Deprecation != nil {
		return true
	}
	// Skip embedding models — they don't support chat
	if strings.Contains(am.ID, "embed") {
		return true
	}
	return false
}

func inferFamily(id string) string {
	switch {
	case strings.HasPrefix(id, "mistral-large"):
		return "mistral-large"
	case strings.HasPrefix(id, "mistral-medium"):
		return "mistral-medium"
	case strings.HasPrefix(id, "mistral-small"):
		return "mistral-small"
	case strings.HasPrefix(id, "mistral-tiny"), strings.HasPrefix(id, "open-mistral-7b"):
		return "mistral-tiny"
	case strings.HasPrefix(id, "codestral"):
		return "codestral"
	case strings.HasPrefix(id, "pixtral"):
		return "pixtral"
	case strings.HasPrefix(id, "ministral"):
		return "ministral"
	case strings.Contains(id, "mixtral"):
		return "mixtral"
	case strings.HasPrefix(id, "open-mistral-nemo"), strings.HasPrefix(id, "mistral-nemo"):
		return "mistral-nemo"
	default:
		return "mistral-other"
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

func buildCapabilities(caps apiModelCapabilities) []string {
	var result []string
	if caps.CompletionChat {
		result = append(result, "chat")
	}
	if caps.FunctionCalling {
		result = append(result, "function_calling")
	}
	if caps.Vision {
		result = append(result, "vision")
	}
	if caps.CompletionFIM {
		result = append(result, "fill_in_middle")
	}
	// All chat models support streaming
	if caps.CompletionChat {
		result = append(result, "streaming")
	}
	return result
}

func inferModalities(caps apiModelCapabilities) adapter.Modalities {
	input := []string{"text"}
	if caps.Vision {
		input = append(input, "image")
	}
	return adapter.Modalities{
		Input:  input,
		Output: []string{"text"},
	}
}

func inferMaxCompletion(id string, contextLength int) int {
	// Mistral doesn't expose max output tokens directly;
	// use sensible defaults based on model tier
	if contextLength >= 128000 {
		return 16384
	}
	if contextLength >= 32000 {
		return 8192
	}
	return 4096
}
