package stepfun

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
	adapter.Register(&StepFun{})
}

// StepFun adapter discovers models from the StepFun API (OpenAI-compatible).
type StepFun struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (s *StepFun) Name() string { return "stepfun" }

func (s *StepFun) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (s *StepFun) Configure(apiKey, baseURL string, client *httpclient.Client) {
	s.apiKey = apiKey
	s.baseURL = baseURL
	s.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (s *StepFun) HealthCheck(ctx context.Context) error {
	url := s.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + s.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := s.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for StepFun.
func (s *StepFun) MinExpectedModels() int { return 1 }

func (s *StepFun) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := s.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("stepfun API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("stepfun docs source not yet implemented")
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

func (s *StepFun) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := s.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + s.apiKey,
	}

	resp, err := s.client.Get(ctx, url, headers)
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

	slog.Info("stepfun API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
	return models, nil
}

func apiModelToDiscovered(am apiModel) *adapter.DiscoveredModel {
	if shouldSkip(am.ID) {
		return nil
	}

	return &adapter.DiscoveredModel{
		Name:         am.ID,
		DisplayName:  inferDisplayName(am.ID),
		Family:       "step",
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
	if strings.Contains(lower, "image") && !strings.Contains(lower, "vision") {
		return true
	}
	return false
}

func inferDisplayName(id string) string {
	overrides := map[string]string{
		"step-1-8k":    "Step 1 8K",
		"step-1-32k":   "Step 1 32K",
		"step-1-128k":  "Step 1 128K",
		"step-1-256k":  "Step 1 256K",
		"step-1v-8k":   "Step 1V 8K",
		"step-1v-32k":  "Step 1V 32K",
		"step-2-16k":   "Step 2 16K",
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
	if strings.Contains(lower, "1v") || strings.Contains(lower, "vision") {
		caps = append(caps, "vision")
	}
	return caps
}

func inferLimits(id string) adapter.Limits {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "256k"):
		return adapter.Limits{MaxTokens: 262144, MaxCompletionTokens: 8192}
	case strings.Contains(lower, "128k"):
		return adapter.Limits{MaxTokens: 131072, MaxCompletionTokens: 8192}
	case strings.Contains(lower, "32k"):
		return adapter.Limits{MaxTokens: 32768, MaxCompletionTokens: 8192}
	case strings.Contains(lower, "16k"):
		return adapter.Limits{MaxTokens: 16384, MaxCompletionTokens: 4096}
	default:
		return adapter.Limits{MaxTokens: 8192, MaxCompletionTokens: 4096}
	}
}

func inferModalities(id string) adapter.Modalities {
	lower := strings.ToLower(id)
	input := []string{"text"}
	if strings.Contains(lower, "1v") || strings.Contains(lower, "vision") {
		input = append(input, "image")
	}
	return adapter.Modalities{
		Input:  input,
		Output: []string{"text"},
	}
}
