package nvidia

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
	adapter.Register(&NVIDIA{})
}

// NVIDIA adapter discovers models from the NVIDIA NIM API (OpenAI-compatible).
type NVIDIA struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (n *NVIDIA) Name() string { return "nvidia" }

func (n *NVIDIA) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (n *NVIDIA) Configure(apiKey, baseURL string, client *httpclient.Client) {
	n.apiKey = apiKey
	n.baseURL = baseURL
	n.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (n *NVIDIA) HealthCheck(ctx context.Context) error {
	url := n.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + n.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := n.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for NVIDIA.
func (n *NVIDIA) MinExpectedModels() int { return 10 }

func (n *NVIDIA) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := n.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("nvidia API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("nvidia docs source not yet implemented")
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

func (n *NVIDIA) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
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

	slog.Info("nvidia API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
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
	// Skip non-chat models aggressively
	if strings.Contains(lower, "embed") {
		return true
	}
	if strings.Contains(lower, "rerank") {
		return true
	}
	if strings.Contains(lower, "nemo") && !strings.Contains(lower, "chat") && !strings.Contains(lower, "instruct") {
		return true
	}
	if strings.Contains(lower, "tts") || strings.Contains(lower, "speech") {
		return true
	}
	if strings.Contains(lower, "audio") && !strings.Contains(lower, "chat") {
		return true
	}
	if strings.Contains(lower, "stable-diffusion") || strings.Contains(lower, "sdxl") {
		return true
	}
	if strings.Contains(lower, "image") && !strings.Contains(lower, "vision") {
		return true
	}
	if strings.Contains(lower, "video") {
		return true
	}
	if strings.Contains(lower, "parakeet") || strings.Contains(lower, "canary") {
		return true
	}
	if strings.Contains(lower, "grounding") || strings.Contains(lower, "segmentation") {
		return true
	}
	if strings.Contains(lower, "nv-") && !strings.Contains(lower, "chat") && !strings.Contains(lower, "llama") {
		return true
	}
	return false
}

// stripOrg removes the org/ prefix from model IDs.
func stripOrg(id string) string {
	parts := strings.Split(id, "/")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return id
}

func inferFamily(id string) string {
	model := strings.ToLower(stripOrg(id))
	switch {
	case strings.Contains(model, "llama-3.3"):
		return "llama-3.3"
	case strings.Contains(model, "llama-3.2"):
		return "llama-3.2"
	case strings.Contains(model, "llama-3.1"):
		return "llama-3.1"
	case strings.Contains(model, "llama"):
		return "llama"
	case strings.Contains(model, "mixtral"):
		return "mixtral"
	case strings.Contains(model, "mistral"):
		return "mistral"
	case strings.Contains(model, "nemotron"):
		return "nemotron"
	case strings.Contains(model, "qwen"):
		return "qwen"
	case strings.Contains(model, "deepseek"):
		return "deepseek"
	case strings.Contains(model, "gemma"):
		return "gemma"
	case strings.Contains(model, "phi"):
		return "phi"
	default:
		return "nvidia-other"
	}
}

func inferDisplayName(id string) string {
	model := stripOrg(id)
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
	lower := strings.ToLower(id)
	if strings.Contains(lower, "vision") || strings.Contains(lower, "vlm") {
		caps = append(caps, "vision")
	}
	caps = append(caps, "function_calling")
	return caps
}

func inferLimits(id string) adapter.Limits {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "llama-3.1") || strings.Contains(lower, "llama-3.3"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	case strings.Contains(lower, "nemotron"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	default:
		return adapter.Limits{MaxTokens: 32768, MaxCompletionTokens: 4096}
	}
}

func inferModalities(id string) adapter.Modalities {
	lower := strings.ToLower(id)
	input := []string{"text"}
	if strings.Contains(lower, "vision") || strings.Contains(lower, "vlm") {
		input = append(input, "image")
	}
	return adapter.Modalities{
		Input:  input,
		Output: []string{"text"},
	}
}
