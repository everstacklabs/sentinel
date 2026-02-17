package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWriteNewModel(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewWriter(tmpDir)

	m := &Model{
		Name:         "gpt-5",
		DisplayName:  "GPT-5",
		Family:       "gpt-5",
		Status:       "stable",
		Capabilities: []string{"chat", "function_calling", "vision"},
		Limits:       Limits{MaxTokens: 128000, MaxCompletionTokens: 16384},
		Modalities:   Modalities{Input: []string{"text", "image"}, Output: []string{"text"}},
	}

	result, err := w.WriteModel("openai", m)
	if err != nil {
		t.Fatalf("WriteModel failed: %v", err)
	}
	if !result.IsNew {
		t.Error("expected IsNew to be true")
	}

	// Verify file exists and is valid YAML
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}

	var loaded Model
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("parsing written YAML: %v", err)
	}
	if loaded.Name != "gpt-5" {
		t.Errorf("loaded name = %q, want gpt-5", loaded.Name)
	}
	if loaded.Limits.MaxTokens != 128000 {
		t.Errorf("loaded max_tokens = %d, want 128000", loaded.Limits.MaxTokens)
	}
}

func TestWriteUpdatedModelPreservesManualFields(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "providers", "openai", "models")
	os.MkdirAll(modelsDir, 0o755)

	// Write an existing file with a manual field (x_updater)
	existingYAML := `name: gpt-4o
display_name: GPT-4O
family: gpt-4
status: stable
custom_notes: "manually added field"
capabilities:
    - chat
    - function_calling
limits:
    max_tokens: 128000
modalities:
    input:
        - text
    output:
        - text
`
	existingPath := filepath.Join(modelsDir, "gpt-4o.yaml")
	os.WriteFile(existingPath, []byte(existingYAML), 0o644)

	w := NewWriter(tmpDir)

	// Discovered model has updated display_name but no custom_notes
	discovered := &Model{
		Name:         "gpt-4o",
		DisplayName:  "GPT-4O Updated",
		Family:       "gpt-4",
		Status:       "stable",
		Capabilities: []string{"chat", "function_calling"},
		Limits:       Limits{MaxTokens: 128000},
		Modalities:   Modalities{Input: []string{"text"}, Output: []string{"text"}},
	}

	result, err := w.WriteModel("openai", discovered)
	if err != nil {
		t.Fatalf("WriteModel failed: %v", err)
	}
	if result.IsNew {
		t.Error("expected IsNew to be false")
	}
	if len(result.Changes) == 0 {
		t.Error("expected at least one change")
	}

	// Verify custom_notes is preserved
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("reading merged file: %v", err)
	}
	if !strings.Contains(string(data), "custom_notes") {
		t.Error("custom_notes should be preserved after merge")
	}
	if !strings.Contains(string(data), "GPT-4O Updated") {
		t.Error("display_name should be updated to GPT-4O Updated")
	}
}

func TestWriteUpdatedModelPreservesFieldOrdering(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "providers", "openai", "models")
	os.MkdirAll(modelsDir, 0o755)

	// Original ordering: name, display_name, family, status, limits
	existingYAML := `name: gpt-4o
display_name: GPT-4O
family: gpt-4
status: stable
limits:
    max_tokens: 128000
capabilities:
    - chat
modalities:
    input:
        - text
    output:
        - text
`
	existingPath := filepath.Join(modelsDir, "gpt-4o.yaml")
	os.WriteFile(existingPath, []byte(existingYAML), 0o644)

	w := NewWriter(tmpDir)
	discovered := &Model{
		Name:         "gpt-4o",
		DisplayName:  "GPT-4O New",
		Family:       "gpt-4",
		Status:       "beta",
		Capabilities: []string{"chat"},
		Limits:       Limits{MaxTokens: 128000},
		Modalities:   Modalities{Input: []string{"text"}, Output: []string{"text"}},
	}

	result, err := w.WriteModel("openai", discovered)
	if err != nil {
		t.Fatalf("WriteModel failed: %v", err)
	}

	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	nameIdx := strings.Index(content, "name:")
	displayIdx := strings.Index(content, "display_name:")
	familyIdx := strings.Index(content, "family:")
	statusIdx := strings.Index(content, "status:")

	if nameIdx >= displayIdx || displayIdx >= familyIdx || familyIdx >= statusIdx {
		t.Error("field ordering not preserved: name should come before display_name, family, status")
	}
}

func TestWriteNoChangesSkipsWrite(t *testing.T) {
	tmpDir := t.TempDir()
	modelsDir := filepath.Join(tmpDir, "providers", "openai", "models")
	os.MkdirAll(modelsDir, 0o755)

	existingYAML := `name: gpt-4o
display_name: GPT-4O
family: gpt-4
status: stable
capabilities:
    - chat
limits:
    max_tokens: 128000
modalities:
    input:
        - text
    output:
        - text
`
	existingPath := filepath.Join(modelsDir, "gpt-4o.yaml")
	os.WriteFile(existingPath, []byte(existingYAML), 0o644)

	w := NewWriter(tmpDir)
	// Same data — no changes
	discovered := &Model{
		Name:         "gpt-4o",
		DisplayName:  "GPT-4O",
		Family:       "gpt-4",
		Status:       "stable",
		Capabilities: []string{"chat"},
		Limits:       Limits{MaxTokens: 128000},
		Modalities:   Modalities{Input: []string{"text"}, Output: []string{"text"}},
	}

	result, err := w.WriteModel("openai", discovered)
	if err != nil {
		t.Fatalf("WriteModel failed: %v", err)
	}
	if result.IsNew {
		t.Error("should not be new")
	}
	if len(result.Changes) != 0 {
		t.Errorf("expected 0 changes, got %d: %v", len(result.Changes), result.Changes)
	}
}

func TestMergeNodesPreservesDstKeysNotInSrc(t *testing.T) {
	dstYAML := `name: gpt-4o
custom_field: keep me
status: stable
`
	srcYAML := `name: gpt-4o
status: beta
`
	var dst, src yaml.Node
	yaml.Unmarshal([]byte(dstYAML), &dst)
	yaml.Unmarshal([]byte(srcYAML), &src)

	merged := mergeNodes(&dst, &src)

	// Serialize back and check custom_field is preserved
	out, _ := yaml.Marshal(merged)
	content := string(out)

	if !strings.Contains(content, "custom_field") {
		t.Error("custom_field should be preserved from dst")
	}
	if !strings.Contains(content, "beta") {
		t.Error("status should be updated to beta from src")
	}
}

func TestComputeChanges(t *testing.T) {
	existing := &Model{
		Name:         "gpt-4o",
		DisplayName:  "GPT-4O",
		Family:       "gpt-4",
		Status:       "stable",
		Capabilities: []string{"chat"},
		Limits:       Limits{MaxTokens: 128000},
		Cost:         &Cost{InputPer1K: 0.005, OutputPer1K: 0.015},
	}
	discovered := &Model{
		Name:         "gpt-4o",
		DisplayName:  "GPT-4O v2",
		Family:       "gpt-4",
		Status:       "stable",
		Capabilities: []string{"chat", "vision"},
		Limits:       Limits{MaxTokens: 256000},
		Cost:         &Cost{InputPer1K: 0.01, OutputPer1K: 0.015},
	}

	changes := computeChanges(existing, discovered)

	fields := make(map[string]bool)
	for _, c := range changes {
		fields[c.Field] = true
	}

	if !fields["display_name"] {
		t.Error("expected display_name change")
	}
	if !fields["limits.max_tokens"] {
		t.Error("expected limits.max_tokens change")
	}
	if !fields["cost.input_per_1k"] {
		t.Error("expected cost.input_per_1k change")
	}
	if !fields["capabilities"] {
		t.Error("expected capabilities change")
	}
	if fields["cost.output_per_1k"] {
		t.Error("cost.output_per_1k should not change (same value)")
	}
}
