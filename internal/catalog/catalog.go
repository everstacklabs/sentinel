package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Catalog holds all loaded models grouped by provider.
type Catalog struct {
	BasePath  string
	Providers map[string]*ProviderCatalog
	Version   string
}

// ProviderCatalog holds models for a single provider.
type ProviderCatalog struct {
	Provider Provider
	Models   map[string]*Model // keyed by model name
}

// Load reads the entire catalog from disk.
func Load(basePath string) (*Catalog, error) {
	cat := &Catalog{
		BasePath:  basePath,
		Providers: make(map[string]*ProviderCatalog),
	}

	// Read version
	versionBytes, err := os.ReadFile(filepath.Join(basePath, "version.txt"))
	if err != nil {
		return nil, fmt.Errorf("reading version.txt: %w", err)
	}
	cat.Version = strings.TrimSpace(string(versionBytes))

	// Scan providers directory
	providersDir := filepath.Join(basePath, "providers")
	entries, err := os.ReadDir(providersDir)
	if err != nil {
		return nil, fmt.Errorf("reading providers dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		providerName := entry.Name()
		pc, err := loadProvider(providersDir, providerName)
		if err != nil {
			return nil, fmt.Errorf("loading provider %s: %w", providerName, err)
		}
		cat.Providers[providerName] = pc
	}

	return cat, nil
}

func loadProvider(providersDir, name string) (*ProviderCatalog, error) {
	providerDir := filepath.Join(providersDir, name)
	pc := &ProviderCatalog{
		Models: make(map[string]*Model),
	}

	// Load provider.yaml
	providerFile := filepath.Join(providerDir, "provider.yaml")
	data, err := os.ReadFile(providerFile)
	if err != nil {
		return nil, fmt.Errorf("reading provider.yaml: %w", err)
	}
	if err := yaml.Unmarshal(data, &pc.Provider); err != nil {
		return nil, fmt.Errorf("parsing provider.yaml: %w", err)
	}

	// Load models
	modelsDir := filepath.Join(providerDir, "models")
	if _, err := os.Stat(modelsDir); os.IsNotExist(err) {
		return pc, nil // Meta-providers may not have models
	}

	modelFiles, err := os.ReadDir(modelsDir)
	if err != nil {
		return nil, fmt.Errorf("reading models dir: %w", err)
	}

	for _, f := range modelFiles {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".yaml") {
			continue
		}

		modelPath := filepath.Join(modelsDir, f.Name())
		data, err := os.ReadFile(modelPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", f.Name(), err)
		}

		var m Model
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", f.Name(), err)
		}
		pc.Models[m.Name] = &m
	}

	return pc, nil
}

// ModelNames returns sorted model names for a provider.
func (c *Catalog) ModelNames(provider string) []string {
	pc, ok := c.Providers[provider]
	if !ok {
		return nil
	}
	names := make([]string, 0, len(pc.Models))
	for name := range pc.Models {
		names = append(names, name)
	}
	return names
}
