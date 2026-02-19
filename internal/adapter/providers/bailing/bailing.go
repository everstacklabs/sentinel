package bailing

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
	adapter.Register(&Bailing{})
}

// Bailing adapter discovers models from the Bailing API (OpenAI-compatible).
type Bailing struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (b *Bailing) Name() string { return "bailing" }

func (b *Bailing) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (b *Bailing) Configure(apiKey, baseURL string, client *httpclient.Client) {
	b.apiKey = apiKey
	b.baseURL = baseURL
	b.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (b *Bailing) HealthCheck(ctx context.Context) error {
	url := b.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + b.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := b.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Bailing.
func (b *Bailing) MinExpectedModels() int { return 1 }

func (b *Bailing) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := b.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("bailing API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("bailing docs source not yet implemented")
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

func (b *Bailing) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := b.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + b.apiKey,
	}

	resp, err := b.client.Get(ctx, url, headers)
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

	slog.Info("bailing API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
	return models, nil
}

func apiModelToDiscovered(am apiModel) *adapter.DiscoveredModel {
	return &adapter.DiscoveredModel{
		Name:         am.ID,
		DisplayName:  inferDisplayName(am.ID),
		Family:       "bailing",
		Status:       "stable",
		Capabilities: []string{"chat", "streaming"},
		Limits:       adapter.Limits{MaxTokens: 32768, MaxCompletionTokens: 4096},
		Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		DiscoveredBy: adapter.SourceAPI,
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
