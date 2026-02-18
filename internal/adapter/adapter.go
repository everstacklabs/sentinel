package adapter

import "context"

// SourceType represents how a model was discovered.
type SourceType string

const (
	SourceAPI  SourceType = "api"
	SourceDocs SourceType = "docs"
	SourceLLM  SourceType = "llm"
)

// DiscoverOptions controls adapter behavior.
type DiscoverOptions struct {
	Sources  []SourceType
	NoCache  bool
	CacheDir string
}

// Adapter discovers models from a provider.
type Adapter interface {
	// Name returns the provider name (e.g., "openai").
	Name() string
	// Discover fetches models from the provider.
	Discover(ctx context.Context, opts DiscoverOptions) ([]DiscoveredModel, error)
	// SupportedSources returns which source types this adapter can use.
	SupportedSources() []SourceType
}

// HealthChecker is an optional interface adapters can implement for pre-discovery
// liveness probes and post-discovery model count validation.
type HealthChecker interface {
	// HealthCheck performs a lightweight liveness probe against the provider.
	HealthCheck(ctx context.Context) error
	// MinExpectedModels returns the minimum number of models expected from this provider.
	// A discovery result below this threshold signals a data quality issue.
	MinExpectedModels() int
}

// DiscoveredModel matches the existing catalog YAML schema.
type DiscoveredModel struct {
	Name         string     `yaml:"name"`
	DisplayName  string     `yaml:"display_name"`
	Family       string     `yaml:"family"`
	Status       string     `yaml:"status"`
	Cost         *Cost      `yaml:"cost,omitempty"`
	Limits       Limits     `yaml:"limits"`
	Capabilities []string   `yaml:"capabilities"`
	Modalities   Modalities `yaml:"modalities"`
	DiscoveredBy SourceType `yaml:"-"` // For PR metadata only, not written to YAML
}

// Cost represents model pricing.
type Cost struct {
	InputPer1K  float64 `yaml:"input_per_1k"`
	OutputPer1K float64 `yaml:"output_per_1k"`
}

// Limits represents model token limits.
type Limits struct {
	MaxTokens           int `yaml:"max_tokens"`
	MaxCompletionTokens int `yaml:"max_completion_tokens,omitempty"`
}

// Modalities represents input/output modalities.
type Modalities struct {
	Input  []string `yaml:"input"`
	Output []string `yaml:"output"`
}
