package inception

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
	adapter.Register(&Inception{})
}

// Inception adapter discovers models from the Inception Labs API (OpenAI-compatible).
type Inception struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (i *Inception) Name() string { return "inception" }

func (i *Inception) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (i *Inception) Configure(apiKey, baseURL string, client *httpclient.Client) {
	i.apiKey = apiKey
	i.baseURL = baseURL
	i.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (i *Inception) HealthCheck(ctx context.Context) error {
	url := i.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + i.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := i.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Inception.
func (i *Inception) MinExpectedModels() int { return 1 }

func (i *Inception) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := i.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("inception API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			slog.Debug("inception docs source not yet implemented")
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

func (i *Inception) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	url := i.baseURL + "/models"
	headers := map[string]string{
		"Authorization": "Bearer " + i.apiKey,
	}

	resp, err := i.client.Get(ctx, url, headers)
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

	slog.Info("inception API discovery complete", "total_api_models", len(modelsResp.Data), "catalog_models", len(models))
	return models, nil
}

func apiModelToDiscovered(am apiModel) *adapter.DiscoveredModel {
	return &adapter.DiscoveredModel{
		Name:         am.ID,
		DisplayName:  inferDisplayName(am.ID),
		Family:       inferFamily(am.ID),
		Status:       "stable",
		Capabilities: inferCapabilities(am.ID),
		Limits:       adapter.Limits{MaxTokens: 32768, MaxCompletionTokens: 8192},
		Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		DiscoveredBy: adapter.SourceAPI,
	}
}

func inferFamily(id string) string {
	lower := strings.ToLower(id)
	if strings.Contains(lower, "mercury") {
		return "mercury"
	}
	return "inception-other"
}

func inferDisplayName(id string) string {
	overrides := map[string]string{
		"mercury-coder-small": "Mercury Coder Small",
		"mercury-coder-large": "Mercury Coder Large",
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
	caps := []string{"chat", "streaming"}
	lower := strings.ToLower(id)
	if strings.Contains(lower, "coder") {
		caps = append(caps, "fill_in_middle")
	}
	return caps
}
