package validate

import (
	"testing"

	"github.com/everstacklabs/sentinel/internal/catalog"
)

func validModel() *catalog.Model {
	return &catalog.Model{
		Name:         "gpt-4o",
		DisplayName:  "GPT-4O",
		Family:       "gpt-4",
		Status:       "stable",
		Capabilities: []string{"chat", "function_calling", "vision"},
		Limits:       catalog.Limits{MaxTokens: 128000, MaxCompletionTokens: 16384},
		Modalities:   catalog.Modalities{Input: []string{"text", "image"}, Output: []string{"text"}},
		Cost:         &catalog.Cost{InputPer1K: 0.005, OutputPer1K: 0.015},
	}
}

func TestValidModelPassesAllChecks(t *testing.T) {
	m := validModel()
	r := ValidateModel(m, "gpt-4o.yaml")

	if r.HasErrors() {
		t.Errorf("expected no errors, got: %v", r.Errors())
	}
	if len(r.Warnings()) > 0 {
		t.Errorf("expected no warnings, got: %v", r.Warnings())
	}
}

func TestMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*catalog.Model)
		errField string
	}{
		{"missing name", func(m *catalog.Model) { m.Name = "" }, "name"},
		{"missing display_name", func(m *catalog.Model) { m.DisplayName = "" }, "display_name"},
		{"missing status", func(m *catalog.Model) { m.Status = "" }, "status"},
		{"zero max_tokens", func(m *catalog.Model) { m.Limits.MaxTokens = 0 }, "limits.max_tokens"},
		{"no capabilities", func(m *catalog.Model) { m.Capabilities = nil }, "capabilities"},
		{"no input modalities", func(m *catalog.Model) { m.Modalities.Input = nil }, "modalities.input"},
		{"no output modalities", func(m *catalog.Model) { m.Modalities.Output = nil }, "modalities.output"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validModel()
			tt.mutate(m)
			r := ValidateModel(m, "gpt-4o.yaml")

			if !r.HasErrors() {
				t.Fatal("expected errors")
			}
			found := false
			for _, e := range r.Errors() {
				if e.Field == tt.errField {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error on field %q, got: %v", tt.errField, r.Errors())
			}
		})
	}
}

func TestPricingOutsideRange(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		out   float64
		field string
	}{
		{"input too high", 0.20, 0.01, "cost.input_per_1k"},
		{"input negative", -0.01, 0.01, "cost.input_per_1k"},
		{"output too high", 0.01, 0.20, "cost.output_per_1k"},
		{"output negative", 0.01, -0.01, "cost.output_per_1k"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validModel()
			m.Cost = &catalog.Cost{InputPer1K: tt.input, OutputPer1K: tt.out}
			r := ValidateModel(m, "gpt-4o.yaml")

			found := false
			for _, e := range r.Errors() {
				if e.Field == tt.field {
					found = true
				}
			}
			if !found {
				t.Errorf("expected error on %q", tt.field)
			}
		})
	}
}

func TestLimitsOutsideRange(t *testing.T) {
	tests := []struct {
		name      string
		maxTokens int
		caps      []string
		wantErr   bool
	}{
		{"chat model below 1024", 512, []string{"chat"}, true},
		{"chat model above 2M", 3_000_000, []string{"chat"}, true},
		{"chat model at 1024", 1024, []string{"chat"}, false},
		{"embedding below 64", 32, []string{"embeddings"}, true},
		{"embedding at 64", 64, []string{"embeddings"}, false},
		{"embedding above 2M", 3_000_000, []string{"embeddings"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validModel()
			m.Limits.MaxTokens = tt.maxTokens
			m.Capabilities = tt.caps
			r := ValidateModel(m, "gpt-4o.yaml")

			hasLimitErr := false
			for _, e := range r.Errors() {
				if e.Field == "limits.max_tokens" {
					hasLimitErr = true
				}
			}
			if tt.wantErr && !hasLimitErr {
				t.Error("expected limits.max_tokens error")
			}
			if !tt.wantErr && hasLimitErr {
				t.Error("unexpected limits.max_tokens error")
			}
		})
	}
}

func TestMaxCompletionTokensExceedsMaxTokens(t *testing.T) {
	m := validModel()
	m.Limits.MaxTokens = 128000
	m.Limits.MaxCompletionTokens = 200000
	r := ValidateModel(m, "gpt-4o.yaml")

	found := false
	for _, e := range r.Errors() {
		if e.Field == "limits.max_completion_tokens" {
			found = true
		}
	}
	if !found {
		t.Error("expected error for max_completion_tokens > max_tokens")
	}
}

func TestNamespacedModelNameMatchesFilename(t *testing.T) {
	m := validModel()
	m.Name = "openai/gpt-4o"
	r := ValidateModel(m, "gpt-4o.yaml")

	for _, e := range r.Errors() {
		if e.Field == "name" && e.Message != "" {
			t.Errorf("namespaced name should match filename, got error: %s", e.Message)
		}
	}
}

func TestNameFileMismatch(t *testing.T) {
	m := validModel()
	m.Name = "gpt-4o"
	r := ValidateModel(m, "gpt-4-turbo.yaml")

	found := false
	for _, e := range r.Errors() {
		if e.Field == "name" {
			found = true
		}
	}
	if !found {
		t.Error("expected name/filename mismatch error")
	}
}

func TestUnknownCapabilitiesProduceWarnings(t *testing.T) {
	m := validModel()
	m.Capabilities = []string{"chat", "quantum_compute"}
	r := ValidateModel(m, "gpt-4o.yaml")

	if r.HasErrors() {
		// Filter out non-capability errors
		for _, e := range r.Errors() {
			if e.Field == "capabilities" {
				t.Error("unknown capabilities should warn, not error")
			}
		}
	}
	found := false
	for _, w := range r.Warnings() {
		if w.Field == "capabilities" {
			found = true
		}
	}
	if !found {
		t.Error("expected warning for unknown capability")
	}
}

func TestUnknownModalitiesProduceWarnings(t *testing.T) {
	m := validModel()
	m.Modalities.Input = []string{"text", "hologram"}
	r := ValidateModel(m, "gpt-4o.yaml")

	found := false
	for _, w := range r.Warnings() {
		if w.Field == "modalities.input" {
			found = true
		}
	}
	if !found {
		t.Error("expected warning for unknown input modality")
	}
}

func TestValidStatusValues(t *testing.T) {
	for _, status := range []string{"stable", "beta", "deprecated", "preview"} {
		t.Run(status, func(t *testing.T) {
			m := validModel()
			m.Status = status
			r := ValidateModel(m, "gpt-4o.yaml")

			for _, w := range r.Warnings() {
				if w.Field == "status" {
					t.Errorf("status %q should be valid, got warning: %s", status, w.Message)
				}
			}
		})
	}
}

func TestEmbeddingZeroOutputCostNoWarning(t *testing.T) {
	m := validModel()
	m.Capabilities = []string{"embeddings"}
	m.Modalities = catalog.Modalities{Input: []string{"text"}, Output: []string{"embedding"}}
	m.Cost = &catalog.Cost{InputPer1K: 0.0001, OutputPer1K: 0}
	m.Limits.MaxTokens = 8191
	r := ValidateModel(m, "gpt-4o.yaml")

	for _, w := range r.Warnings() {
		if w.Field == "cost.output_per_1k" {
			t.Error("embedding model should not warn about zero output cost")
		}
	}
}

func TestNonEmbeddingZeroOutputCostWarns(t *testing.T) {
	m := validModel()
	m.Cost = &catalog.Cost{InputPer1K: 0.005, OutputPer1K: 0}
	r := ValidateModel(m, "gpt-4o.yaml")

	found := false
	for _, w := range r.Warnings() {
		if w.Field == "cost.output_per_1k" {
			found = true
		}
	}
	if !found {
		t.Error("expected warning for non-embedding model with zero output cost")
	}
}

func TestFormatResultNoIssues(t *testing.T) {
	r := &Result{}
	s := FormatResult(r)
	if s != "Validation passed: no issues found." {
		t.Errorf("unexpected format: %s", s)
	}
}
