package google

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/httpclient"
)

func init() {
	adapter.Register(&Google{})
}

// Google adapter discovers models from the Gemini API.
type Google struct {
	apiKey  string
	baseURL string
	client  *httpclient.Client
}

func (g *Google) Name() string { return "google" }

func (g *Google) SupportedSources() []adapter.SourceType {
	return []adapter.SourceType{adapter.SourceAPI}
}

// Configure sets up the adapter with API credentials and HTTP client.
func (g *Google) Configure(apiKey, baseURL string, client *httpclient.Client) {
	g.apiKey = apiKey
	g.baseURL = baseURL
	g.client = client
}

// HealthCheck performs a lightweight GET to the models endpoint.
func (g *Google) HealthCheck(ctx context.Context) error {
	if g.apiKey == "" {
		slog.Warn("google api key missing; skipping health check")
		return nil
	}
	url := g.baseURL + "/models?pageSize=1&key=" + g.apiKey
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := g.client.Get(ctx, url, nil)
	return err
}

// MinExpectedModels returns the minimum model count for Google.
func (g *Google) MinExpectedModels() int { return 5 }

func (g *Google) Discover(ctx context.Context, opts adapter.DiscoverOptions) ([]adapter.DiscoveredModel, error) {
	var models []adapter.DiscoveredModel

	for _, src := range opts.Sources {
		switch src {
		case adapter.SourceAPI:
			apiModels, err := g.discoverFromAPI(ctx)
			if err != nil {
				return nil, fmt.Errorf("google API discovery: %w", err)
			}
			models = append(models, apiModels...)
		case adapter.SourceDocs:
			docModels, err := g.discoverFromDocs(ctx)
			if err != nil {
				return nil, fmt.Errorf("google docs discovery: %w", err)
			}
			models = append(models, docModels...)
		}
	}

	return models, nil
}

// Gemini /v1beta/models response types.
type modelsResponse struct {
	Models        []apiModel `json:"models"`
	NextPageToken string     `json:"nextPageToken"`
}

type apiModel struct {
	Name                       string   `json:"name"`
	BaseModelID                string   `json:"baseModelId"`
	Version                    string   `json:"version"`
	DisplayName                string   `json:"displayName"`
	Description                string   `json:"description"`
	InputTokenLimit            int      `json:"inputTokenLimit"`
	OutputTokenLimit           int      `json:"outputTokenLimit"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

func (g *Google) discoverFromAPI(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	if g.apiKey == "" {
		slog.Warn("google api key missing; skipping API discovery")
		return nil, nil
	}
	primaryModels, err := g.listModelsFromAPI(ctx, g.baseURL)
	if err != nil {
		return nil, err
	}

	allAPIModels := primaryModels
	if strings.HasSuffix(g.baseURL, "/v1beta") {
		v1BaseURL := strings.TrimSuffix(g.baseURL, "/v1beta") + "/v1"
		v1Models, err := g.listModelsFromAPI(ctx, v1BaseURL)
		if err != nil {
			slog.Warn("google API v1 discovery failed", "error", err)
		} else {
			allAPIModels = mergeAPIModels(primaryModels, v1Models)
		}
	}

	var models []adapter.DiscoveredModel
	for _, am := range allAPIModels {
		m := g.apiModelToDiscovered(am)
		if m != nil {
			models = append(models, *m)
		}
	}

	slog.Info("google API discovery complete", "total_api_models", len(allAPIModels), "catalog_models", len(models))
	return models, nil
}

func (g *Google) listModelsFromAPI(ctx context.Context, baseURL string) ([]apiModel, error) {
	var allAPIModels []apiModel
	pageToken := ""

	for {
		url := baseURL + "/models?pageSize=1000&key=" + g.apiKey
		if pageToken != "" {
			url += "&pageToken=" + pageToken
		}

		resp, err := g.client.Get(ctx, url, nil)
		if err != nil {
			return nil, err
		}

		var modelsResp modelsResponse
		if err := json.Unmarshal(resp.Body, &modelsResp); err != nil {
			return nil, fmt.Errorf("parsing models response: %w", err)
		}

		allAPIModels = append(allAPIModels, modelsResp.Models...)

		if modelsResp.NextPageToken == "" {
			break
		}
		pageToken = modelsResp.NextPageToken
	}

	return allAPIModels, nil
}

func mergeAPIModels(primary, secondary []apiModel) []apiModel {
	if len(secondary) == 0 {
		return primary
	}
	byName := make(map[string]apiModel, len(primary)+len(secondary))
	order := make([]string, 0, len(primary)+len(secondary))

	for _, m := range primary {
		if _, ok := byName[m.Name]; !ok {
			order = append(order, m.Name)
		}
		byName[m.Name] = m
	}
	for _, m := range secondary {
		if _, ok := byName[m.Name]; !ok {
			order = append(order, m.Name)
			byName[m.Name] = m
		}
	}

	result := make([]apiModel, 0, len(order))
	for _, name := range order {
		result = append(result, byName[name])
	}
	return result
}

// apiModelToDiscovered converts a Gemini API model to a DiscoveredModel.
// Returns nil for models we don't want in the catalog.
func (g *Google) apiModelToDiscovered(am apiModel) *adapter.DiscoveredModel {
	// The API returns names like "models/gemini-2.0-flash"
	id := strings.TrimPrefix(am.Name, "models/")

	if shouldSkip(id, am.SupportedGenerationMethods) {
		return nil
	}

	family := inferFamily(id)
	displayName := am.DisplayName
	if displayName == "" {
		displayName = inferDisplayName(id)
	}
	capabilities := inferCapabilities(id, am.SupportedGenerationMethods)
	modalities := inferModalities(id)
	limits := adapter.Limits{
		MaxTokens:           am.InputTokenLimit,
		MaxCompletionTokens: am.OutputTokenLimit,
	}

	return &adapter.DiscoveredModel{
		Name:         id,
		DisplayName:  displayName,
		Family:       family,
		Status:       "stable",
		Capabilities: capabilities,
		Limits:       limits,
		Modalities:   modalities,
		DiscoveredBy: adapter.SourceAPI,
	}
}

// datedSnapshotRe matches dated/versioned snapshot IDs like gemini-2.0-flash-001.
var datedSnapshotRe = regexp.MustCompile(`-\d{3}$`)

func shouldSkip(id string, methods []string) bool {
	// Skip versioned snapshots (e.g. gemini-1.5-flash-001, gemini-2.0-flash-001)
	if datedSnapshotRe.MatchString(id) {
		return true
	}

	// Skip legacy/non-generative models
	skipPrefixes := []string{"chat-bison", "text-bison", "embedding-", "aqa"}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}

	// Skip models that don't support content generation
	hasGenerate := false
	for _, m := range methods {
		if m == "generateContent" {
			hasGenerate = true
			break
		}
	}
	return !hasGenerate
}

func inferFamily(id string) string {
	switch {
	case strings.HasPrefix(id, "gemini-3"):
		return "gemini-3"
	case strings.HasPrefix(id, "gemini-2"):
		return "gemini-2"
	case strings.HasPrefix(id, "gemini-1.5"):
		return "gemini-1.5"
	case strings.HasPrefix(id, "gemini-1.0"), strings.HasPrefix(id, "gemini-pro"):
		return "gemini-1.0"
	case strings.HasPrefix(id, "gemma"):
		return "gemma"
	default:
		return "google-other"
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

func inferCapabilities(id string, methods []string) []string {
	caps := []string{"chat"}

	// Function calling — available for Gemini models (not Gemma)
	if !strings.HasPrefix(id, "gemma") {
		caps = append(caps, "function_calling")
	}

	// Vision — Gemini models support multimodal input
	if !strings.HasPrefix(id, "gemma") {
		caps = append(caps, "vision")
	}

	// Streaming — check if streamGenerateContent is supported
	for _, m := range methods {
		if m == "streamGenerateContent" {
			caps = append(caps, "streaming")
			break
		}
	}

	// Thinking — Gemini 2.x flash-thinking models
	if strings.Contains(id, "thinking") {
		caps = append(caps, "thinking")
	}

	return caps
}

func inferModalities(id string) adapter.Modalities {
	// Gemma models are text-only
	if strings.HasPrefix(id, "gemma") {
		return adapter.Modalities{
			Input:  []string{"text"},
			Output: []string{"text"},
		}
	}

	// Gemini models support multimodal input
	return adapter.Modalities{
		Input:  []string{"text", "image", "video", "audio"},
		Output: []string{"text"},
	}
}
