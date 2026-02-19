package cohere

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
	adapter.Register(&Cohere{})
}

// Cohere adapter discovers models from the Cohere API.
type Cohere struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (c *Cohere) Name() string { return "cohere" }

func (c *Cohere) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI, adapter.SourceDocs}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (c *Cohere) Configure(apiKey, baseURL string, client *httpclient.Client) {
	c.apiKey = apiKey
	c.baseURL = baseURL
	c.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (c *Cohere) HealthCheck(ctx context.Context) error {
	url := c.baseURL + "/models?page_size=1"
	headers := map[string]string{
		"Authorization": "Bearer " + c.apiKey,
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := c.client.Get(ctx, url, headers)
	return err
}

// MinExpectedModels returns the minimum model count for Cohere.
func (c *Cohere) MinExpectedModels() int { return 3 }

func (c *Cohere) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := c.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("cohere API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			docModels, err := c.discoverFromDocs(ctx)
			if err != nil {
				slog.Warn("cohere docs discovery failed, continuing", "error", err)
			} else {
				models = append(models, docModels...)
			}
		}
	}

	return models, nil
}

// Cohere /v2/models response types.
type modelsResponse struct {
	Models        []apiModel `json:"models"`
	NextPageToken string     `json:"next_page_token"`
}

type apiModel struct {
	Name             string   `json:"name"`
	Endpoints        []string `json:"endpoints"`
	ContextLength    int      `json:"context_length"`
	TokenizerURL     string   `json:"tokenizer_url"`
	DefaultEndpoint  string   `json:"default_endpoint"`
}

func (c *Cohere) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + c.apiKey,
	}

	var allModels []apiModel
	pageToken := ""

	for {
		url := c.baseURL + "/models?page_size=100"
		if pageToken != "" {
			url += "&page_token=" + pageToken
		}

		resp, err := c.client.Get(ctx, url, headers)
		if err != nil {
			return nil, err
		}

		var modelsResp modelsResponse
		if err := json.Unmarshal(resp.Body, &modelsResp); err != nil {
			return nil, fmt.Errorf("parsing models response: %w", err)
		}

		allModels = append(allModels, modelsResp.Models...)

		if modelsResp.NextPageToken == "" {
			break
		}
		pageToken = modelsResp.NextPageToken
	}

	var models []adapter.DiscoveredModel
	for _, am := range allModels {
		m := apiModelToDiscovered(am)
		if m != nil {
			models = append(models, *m)
		}
	}

	slog.Info("cohere API discovery complete", "total_api_models", len(allModels), "catalog_models", len(models))
	return models, nil
}

func apiModelToDiscovered(am apiModel) *adapter.DiscoveredModel {
	if shouldSkip(am) {
		return nil
	}

	family := inferFamily(am.Name)
	capabilities := inferCapabilities(am)
	modalities := inferModalities(am)

	return &adapter.DiscoveredModel{
		Name:         am.Name,
		DisplayName:  inferDisplayName(am.Name),
		Family:       family,
		Status:       "stable",
		Capabilities: capabilities,
		Limits:       adapter.Limits{MaxTokens: am.ContextLength, MaxCompletionTokens: inferMaxCompletion(am.ContextLength)},
		Modalities:   modalities,
		DiscoveredBy: adapter.SourceAPI,
	}
}

func shouldSkip(am apiModel) bool {
	// Skip embedding-only models
	if len(am.Endpoints) == 1 && am.Endpoints[0] == "embed" {
		return true
	}
	// Skip rerank-only models
	if len(am.Endpoints) == 1 && am.Endpoints[0] == "rerank" {
		return true
	}
	return false
}

func inferFamily(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "command-r-plus") || strings.HasPrefix(lower, "command-r+"):
		return "command-r-plus"
	case strings.HasPrefix(lower, "command-r"):
		return "command-r"
	case strings.HasPrefix(lower, "command-light"):
		return "command-light"
	case strings.HasPrefix(lower, "command"):
		return "command"
	default:
		return "cohere-other"
	}
}

func inferDisplayName(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func inferCapabilities(am apiModel) []string {
	caps := []string{}
	for _, ep := range am.Endpoints {
		switch ep {
		case "chat":
			caps = append(caps, "chat")
		case "generate":
			caps = append(caps, "completion")
		case "embed":
			caps = append(caps, "embeddings")
		case "rerank":
			caps = append(caps, "rerank")
		}
	}
	// Cohere chat models support function calling and streaming
	for _, c := range caps {
		if c == "chat" {
			caps = append(caps, "function_calling", "streaming")
			break
		}
	}
	return caps
}

func inferModalities(am apiModel) adapter.Modalities {
	return adapter.Modalities{
		Input:  []string{"text"},
		Output: []string{"text"},
	}
}

func inferMaxCompletion(contextLength int) int {
	if contextLength >= 128000 {
		return 4096
	}
	return 4096
}
