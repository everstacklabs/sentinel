package upstage

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
	adapter.Register(&Upstage{})
}

// Upstage adapter discovers models from the Upstage Solar API (OpenAI-compatible).
type Upstage struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (u *Upstage) Name() string { return "upstage" }

func (u *Upstage) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (u *Upstage) Configure(apiKey, baseURL string, client *httpclient.Client) {
	u.apiKey = apiKey
	u.baseURL = baseURL
	u.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (u *Upstage) HealthCheck(ctx context.Context) error {
	url := u.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + u.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := u.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Upstage.
func (u *Upstage) MinExpectedModels() int { return 1 }

func (u *Upstage) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := u.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("upstage API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("upstage docs source not yet implemented")
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

func (u *Upstage) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := u.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + u.apiKey,
	}

	resp, err := u.client.Get(ctx, url, headers)
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

	slog.Info("upstage API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
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
		Capabilities: []string{"chat", "function_calling", "streaming"},
		Limits:       adapter.Limits{MaxTokens: 32768, MaxCompletionTokens: 4096},
		Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		DiscoveredBy: adapter.SourceAPI,
	}
}

func shouldSkip(id string) bool {
	lower := strings.ToLower(id)
	if strings.Contains(lower, "embed") {
		return true
	}
	if strings.Contains(lower, "groundedness") {
		return true
	}
	return false
}

func inferFamily(id string) string {
	lower := strings.ToLower(id)
	if strings.Contains(lower, "solar") {
		return "solar"
	}
	return "upstage-other"
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
