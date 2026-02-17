package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ManifestProvider describes a provider entry in the manifest.
type ManifestProvider struct {
	Name   string   `yaml:"name"`
	Files  []string `yaml:"files"`
	Models []string `yaml:"models,omitempty"`
}

// ManifestStats holds aggregate counts.
type ManifestStats struct {
	TotalProviders int `yaml:"total_providers"`
	TotalModels    int `yaml:"total_models"`
	StaticProviders int `yaml:"static_providers"`
	MetaProviders   int `yaml:"meta_providers"`
}

// Manifest represents the manifest.yaml file.
type Manifest struct {
	Version       string             `yaml:"version"`
	GeneratedAt   string             `yaml:"generated_at"`
	SchemaVersion string             `yaml:"schema_version"`
	Providers     []ManifestProvider `yaml:"providers"`
	Stats         ManifestStats      `yaml:"stats"`
}

// GenerateManifest creates a new manifest.yaml from the catalog on disk.
// This is the Go reimplementation of scripts/generate-manifest.sh.
func GenerateManifest(basePath string) error {
	// Read version
	versionBytes, err := os.ReadFile(filepath.Join(basePath, "version.txt"))
	if err != nil {
		return fmt.Errorf("reading version.txt: %w", err)
	}
	version := strings.TrimSpace(string(versionBytes))

	providersDir := filepath.Join(basePath, "providers")
	entries, err := os.ReadDir(providersDir)
	if err != nil {
		return fmt.Errorf("reading providers dir: %w", err)
	}

	var (
		providers     []ManifestProvider
		totalModels   int
		staticCount   int
		metaCount     int
	)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		providerDir := filepath.Join(providersDir, name)

		mp := ManifestProvider{Name: name}

		// Add standard provider files
		for _, f := range []string{"provider.yaml", "categories.yaml", "templates.yaml"} {
			relPath := filepath.Join("providers", name, f)
			absPath := filepath.Join(basePath, relPath)
			if _, err := os.Stat(absPath); err == nil {
				mp.Files = append(mp.Files, relPath)
			}
		}

		// Detect provider type
		providerYAML := filepath.Join(providerDir, "provider.yaml")
		if data, err := os.ReadFile(providerYAML); err == nil {
			var p Provider
			if yaml.Unmarshal(data, &p) == nil {
				if p.ProviderType == "meta" {
					metaCount++
				} else {
					staticCount++
				}
			}
		}

		// Scan models
		modelsDir := filepath.Join(providerDir, "models")
		if modelEntries, err := os.ReadDir(modelsDir); err == nil {
			var modelFiles []string
			for _, mf := range modelEntries {
				if !mf.IsDir() && strings.HasSuffix(mf.Name(), ".yaml") {
					modelFiles = append(modelFiles, filepath.Join("providers", name, "models", mf.Name()))
				}
			}
			sort.Strings(modelFiles)
			mp.Models = modelFiles
			totalModels += len(modelFiles)
		}

		providers = append(providers, mp)
	}

	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name < providers[j].Name
	})

	manifest := Manifest{
		Version:       version,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		SchemaVersion: "1.0",
		Providers:     providers,
		Stats: ManifestStats{
			TotalProviders:  len(providers),
			TotalModels:     totalModels,
			StaticProviders: staticCount,
			MetaProviders:   metaCount,
		},
	}

	// Write with header comment
	data, err := yaml.Marshal(&manifest)
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	header := "# Model Catalog Manifest\n# Auto-generated - DO NOT EDIT MANUALLY\n# Run: sentinel sync or ./scripts/generate-manifest.sh to regenerate\n\n"
	output := header + string(data)

	return os.WriteFile(filepath.Join(basePath, "manifest.yaml"), []byte(output), 0o644)
}
