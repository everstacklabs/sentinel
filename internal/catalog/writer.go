package catalog

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FieldChange records a single field change for diff reporting.
type FieldChange struct {
	Field    string
	OldValue any
	NewValue any
}

// WriteResult reports what happened when a model was written.
type WriteResult struct {
	Path    string
	IsNew   bool
	Changes []FieldChange
}

// SmartMergeWriter writes model YAML files using smart merge strategy:
// - Preserves manually-added fields not in the discovered model
// - Preserves field ordering from existing file
// - Only updates fields the adapter has authoritative data for
type SmartMergeWriter struct {
	basePath string
}

// NewWriter creates a new SmartMergeWriter.
func NewWriter(basePath string) *SmartMergeWriter {
	return &SmartMergeWriter{basePath: basePath}
}

// WriteModel performs a smart merge of a discovered model into the catalog.
// It loads the existing YAML as a node tree (preserving order and unknown fields),
// overlays the discovered fields, and writes back.
func (w *SmartMergeWriter) WriteModel(provider string, discovered *Model) (*WriteResult, error) {
	modelsDir := filepath.Join(w.basePath, "providers", provider, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating models dir: %w", err)
	}

	filename := discovered.Name + ".yaml"
	filePath := filepath.Join(modelsDir, filename)

	result := &WriteResult{Path: filePath}

	// Check if file exists
	existingData, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		// New model — write fresh
		result.IsNew = true
		return result, w.writeNewModel(filePath, discovered)
	} else if err != nil {
		return nil, fmt.Errorf("reading existing file: %w", err)
	}

	// Smart merge: parse existing as yaml.Node to preserve structure
	var existingDoc yaml.Node
	if err := yaml.Unmarshal(existingData, &existingDoc); err != nil {
		return nil, fmt.Errorf("parsing existing YAML: %w", err)
	}

	// Also parse existing into a Model for comparison
	var existingModel Model
	if err := yaml.Unmarshal(existingData, &existingModel); err != nil {
		return nil, fmt.Errorf("parsing existing model: %w", err)
	}

	// Compute changes
	result.Changes = computeChanges(&existingModel, discovered)
	if len(result.Changes) == 0 {
		return result, nil // No changes needed
	}

	// Merge: serialize discovered to a node, then overlay onto existing
	discoveredData, err := yaml.Marshal(discovered)
	if err != nil {
		return nil, fmt.Errorf("marshaling discovered model: %w", err)
	}

	var discoveredDoc yaml.Node
	if err := yaml.Unmarshal(discoveredData, &discoveredDoc); err != nil {
		return nil, fmt.Errorf("parsing discovered YAML: %w", err)
	}

	merged := mergeNodes(&existingDoc, &discoveredDoc)

	out, err := yaml.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("marshaling merged YAML: %w", err)
	}

	if err := os.WriteFile(filePath, out, 0o644); err != nil {
		return nil, fmt.Errorf("writing merged file: %w", err)
	}

	return result, nil
}

func (w *SmartMergeWriter) writeNewModel(path string, m *Model) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling model: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// mergeNodes overlays src mapping keys onto dst mapping, preserving dst order
// and any keys in dst not present in src.
func mergeNodes(dst, src *yaml.Node) *yaml.Node {
	// Handle document nodes
	if dst.Kind == yaml.DocumentNode && len(dst.Content) > 0 {
		dst = dst.Content[0]
	}
	if src.Kind == yaml.DocumentNode && len(src.Content) > 0 {
		src = src.Content[0]
	}

	if dst.Kind != yaml.MappingNode || src.Kind != yaml.MappingNode {
		return src
	}

	// Build src key→value index
	srcMap := make(map[string]*yaml.Node)
	for i := 0; i+1 < len(src.Content); i += 2 {
		srcMap[src.Content[i].Value] = src.Content[i+1]
	}

	// Update existing keys in dst order
	seen := make(map[string]bool)
	for i := 0; i+1 < len(dst.Content); i += 2 {
		key := dst.Content[i].Value
		if srcVal, ok := srcMap[key]; ok {
			dst.Content[i+1] = srcVal
			seen[key] = true
		}
	}

	// Append new keys from src not in dst
	for i := 0; i+1 < len(src.Content); i += 2 {
		key := src.Content[i].Value
		if !seen[key] {
			dst.Content = append(dst.Content, src.Content[i], src.Content[i+1])
		}
	}

	return dst
}

func computeChanges(existing, discovered *Model) []FieldChange {
	var changes []FieldChange

	if existing.DisplayName != discovered.DisplayName && discovered.DisplayName != "" {
		changes = append(changes, FieldChange{"display_name", existing.DisplayName, discovered.DisplayName})
	}
	if existing.Family != discovered.Family && discovered.Family != "" {
		changes = append(changes, FieldChange{"family", existing.Family, discovered.Family})
	}
	if existing.Status != discovered.Status && discovered.Status != "" {
		changes = append(changes, FieldChange{"status", existing.Status, discovered.Status})
	}

	// Cost changes
	if discovered.Cost != nil {
		if existing.Cost == nil {
			changes = append(changes, FieldChange{"cost", nil, discovered.Cost})
		} else {
			if existing.Cost.InputPer1K != discovered.Cost.InputPer1K {
				changes = append(changes, FieldChange{"cost.input_per_1k", existing.Cost.InputPer1K, discovered.Cost.InputPer1K})
			}
			if existing.Cost.OutputPer1K != discovered.Cost.OutputPer1K {
				changes = append(changes, FieldChange{"cost.output_per_1k", existing.Cost.OutputPer1K, discovered.Cost.OutputPer1K})
			}
		}
	}

	// Limits changes
	if discovered.Limits.MaxTokens != 0 && existing.Limits.MaxTokens != discovered.Limits.MaxTokens {
		changes = append(changes, FieldChange{"limits.max_tokens", existing.Limits.MaxTokens, discovered.Limits.MaxTokens})
	}
	if discovered.Limits.MaxCompletionTokens != 0 && existing.Limits.MaxCompletionTokens != discovered.Limits.MaxCompletionTokens {
		changes = append(changes, FieldChange{"limits.max_completion_tokens", existing.Limits.MaxCompletionTokens, discovered.Limits.MaxCompletionTokens})
	}

	// Capabilities — check for additions
	existingCaps := toSet(existing.Capabilities)
	for _, cap := range discovered.Capabilities {
		if !existingCaps[cap] {
			changes = append(changes, FieldChange{"capabilities", existing.Capabilities, discovered.Capabilities})
			break
		}
	}

	return changes
}

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}
