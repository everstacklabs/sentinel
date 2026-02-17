package catalog

// Model represents a model YAML file in the catalog.
// Fields match the existing catalog schema exactly.
type Model struct {
	Name         string     `yaml:"name"`
	DisplayName  string     `yaml:"display_name"`
	Family       string     `yaml:"family"`
	Status       string     `yaml:"status"`
	Cost         *Cost      `yaml:"cost,omitempty"`
	Limits       Limits     `yaml:"limits"`
	Capabilities []string   `yaml:"capabilities"`
	Modalities   Modalities `yaml:"modalities"`
	XUpdater     *XUpdater  `yaml:"x_updater,omitempty"`
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

// XUpdater holds updater-specific metadata appended to model files.
type XUpdater struct {
	LastVerifiedAt string   `yaml:"last_verified_at"`
	Sources        []string `yaml:"sources"`
}

// Provider represents a provider.yaml file.
type Provider struct {
	Name                    string `yaml:"name"`
	DisplayName             string `yaml:"display_name"`
	ProviderType            string `yaml:"provider_type"`
	SupportsModelDiscovery  bool   `yaml:"supports_model_discovery"`
}
