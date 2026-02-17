package validate

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/everstacklabs/sentinel/internal/catalog"
)

// Severity classifies validation issues.
type Severity int

const (
	SeverityError   Severity = iota // Blocks PR creation
	SeverityWarning                 // Included in PR body but doesn't block
)

// Issue represents a single validation problem.
type Issue struct {
	Severity Severity
	Model    string
	Field    string
	Message  string
}

func (i Issue) String() string {
	sev := "ERROR"
	if i.Severity == SeverityWarning {
		sev = "WARN"
	}
	return fmt.Sprintf("[%s] %s: %s — %s", sev, i.Model, i.Field, i.Message)
}

// Result holds all validation issues.
type Result struct {
	Issues []Issue
}

// HasErrors returns true if there are any blocking errors.
func (r *Result) HasErrors() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Errors returns only error-severity issues.
func (r *Result) Errors() []Issue {
	var errs []Issue
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			errs = append(errs, i)
		}
	}
	return errs
}

// Warnings returns only warning-severity issues.
func (r *Result) Warnings() []Issue {
	var warns []Issue
	for _, i := range r.Issues {
		if i.Severity == SeverityWarning {
			warns = append(warns, i)
		}
	}
	return warns
}

// Known capability values (warn on unknown, don't block).
var knownCapabilities = map[string]bool{
	"chat":              true,
	"completions":       true,
	"embeddings":        true,
	"function_calling":  true,
	"vision":            true,
	"streaming":         true,
	"fine_tuning":       true,
	"extended_thinking": true,
	"computer_use":      true,
	"reasoning":         true,
	"coding":            true,
	"rerank":            true,
}

// Known modality values.
var knownModalities = map[string]bool{
	"text":      true,
	"image":     true,
	"audio":     true,
	"video":     true,
	"embedding": true,
}

// ValidateModel checks a single model for schema compliance.
func ValidateModel(m *catalog.Model, filename string) *Result {
	r := &Result{}

	// Required fields
	if m.Name == "" {
		r.Issues = append(r.Issues, Issue{SeverityError, filename, "name", "required field is empty"})
	}
	if m.DisplayName == "" {
		r.Issues = append(r.Issues, Issue{SeverityError, filename, "display_name", "required field is empty"})
	}
	if m.Status == "" {
		r.Issues = append(r.Issues, Issue{SeverityError, filename, "status", "required field is empty"})
	}
	if m.Limits.MaxTokens == 0 {
		r.Issues = append(r.Issues, Issue{SeverityError, filename, "limits.max_tokens", "required field is zero"})
	}
	if len(m.Capabilities) == 0 {
		r.Issues = append(r.Issues, Issue{SeverityError, filename, "capabilities", "at least one capability required"})
	}
	if len(m.Modalities.Input) == 0 {
		r.Issues = append(r.Issues, Issue{SeverityError, filename, "modalities.input", "at least one input modality required"})
	}
	if len(m.Modalities.Output) == 0 {
		r.Issues = append(r.Issues, Issue{SeverityError, filename, "modalities.output", "at least one output modality required"})
	}

	// Naming consistency: filename must match name field
	// For namespaced names (e.g., "openai/gpt-4o"), compare against the last segment
	if m.Name != "" && filename != "" {
		actualFilename := filepath.Base(filename)
		// Handle namespaced model names (e.g., "huggingface/gpt-4o" → "gpt-4o.yaml")
		nameForFile := m.Name
		if idx := strings.LastIndex(nameForFile, "/"); idx >= 0 {
			nameForFile = nameForFile[idx+1:]
		}
		expectedFilename := nameForFile + ".yaml"
		if actualFilename != expectedFilename {
			r.Issues = append(r.Issues, Issue{SeverityError, filename, "name",
				fmt.Sprintf("filename %q does not match name field %q", actualFilename, m.Name)})
		}
	}

	// Status values
	validStatuses := map[string]bool{"stable": true, "beta": true, "deprecated": true, "preview": true}
	if m.Status != "" && !validStatuses[m.Status] {
		r.Issues = append(r.Issues, Issue{SeverityWarning, m.Name, "status",
			fmt.Sprintf("unknown status %q, expected one of: stable, beta, deprecated", m.Status)})
	}

	// Check if model is embedding type (used in multiple checks below)
	isEmbedding := false
	for _, cap := range m.Capabilities {
		if cap == "embeddings" {
			isEmbedding = true
			break
		}
	}

	// Pricing sanity
	if m.Cost != nil {
		if m.Cost.InputPer1K < 0 || m.Cost.InputPer1K > 0.10 {
			r.Issues = append(r.Issues, Issue{SeverityError, m.Name, "cost.input_per_1k",
				fmt.Sprintf("value %.6f outside expected range [0, 0.10]", m.Cost.InputPer1K)})
		}
		if m.Cost.OutputPer1K < 0 || m.Cost.OutputPer1K > 0.10 {
			r.Issues = append(r.Issues, Issue{SeverityError, m.Name, "cost.output_per_1k",
				fmt.Sprintf("value %.6f outside expected range [0, 0.10]", m.Cost.OutputPer1K)})
		}
		if !isEmbedding && m.Cost.OutputPer1K == 0 {
			r.Issues = append(r.Issues, Issue{SeverityWarning, m.Name, "cost.output_per_1k",
				"non-embedding model has zero output cost"})
		}
	}

	// Limits sanity — embedding models can have smaller max_tokens
	if m.Limits.MaxTokens > 0 {
		minTokens := 1024
		if isEmbedding {
			minTokens = 64
		}
		if m.Limits.MaxTokens < minTokens || m.Limits.MaxTokens > 2_000_000 {
			r.Issues = append(r.Issues, Issue{SeverityError, m.Name, "limits.max_tokens",
				fmt.Sprintf("value %d outside expected range [%d, 2000000]", m.Limits.MaxTokens, minTokens)})
		}
	}
	if m.Limits.MaxCompletionTokens > 0 && m.Limits.MaxCompletionTokens > m.Limits.MaxTokens {
		r.Issues = append(r.Issues, Issue{SeverityError, m.Name, "limits.max_completion_tokens",
			fmt.Sprintf("value %d exceeds max_tokens %d", m.Limits.MaxCompletionTokens, m.Limits.MaxTokens)})
	}

	// Capability taxonomy
	for _, cap := range m.Capabilities {
		if !knownCapabilities[cap] {
			r.Issues = append(r.Issues, Issue{SeverityWarning, m.Name, "capabilities",
				fmt.Sprintf("unknown capability %q", cap)})
		}
	}

	// Modality taxonomy
	for _, mod := range m.Modalities.Input {
		if !knownModalities[mod] {
			r.Issues = append(r.Issues, Issue{SeverityWarning, m.Name, "modalities.input",
				fmt.Sprintf("unknown modality %q", mod)})
		}
	}
	for _, mod := range m.Modalities.Output {
		if !knownModalities[mod] {
			r.Issues = append(r.Issues, Issue{SeverityWarning, m.Name, "modalities.output",
				fmt.Sprintf("unknown modality %q", mod)})
		}
	}

	return r
}

// ValidateCatalog validates all models in a catalog.
func ValidateCatalog(cat *catalog.Catalog) *Result {
	r := &Result{}
	for providerName, pc := range cat.Providers {
		for modelName, model := range pc.Models {
			filename := filepath.Join("providers", providerName, "models", modelName+".yaml")
			modelResult := ValidateModel(model, filename)
			r.Issues = append(r.Issues, modelResult.Issues...)
		}
	}
	return r
}

// FormatResult formats validation results for display.
func FormatResult(r *Result) string {
	if len(r.Issues) == 0 {
		return "Validation passed: no issues found."
	}

	var b strings.Builder
	errors := r.Errors()
	warnings := r.Warnings()

	if len(errors) > 0 {
		b.WriteString(fmt.Sprintf("Errors (%d):\n", len(errors)))
		for _, e := range errors {
			b.WriteString(fmt.Sprintf("  %s\n", e))
		}
	}

	if len(warnings) > 0 {
		b.WriteString(fmt.Sprintf("Warnings (%d):\n", len(warnings)))
		for _, w := range warnings {
			b.WriteString(fmt.Sprintf("  %s\n", w))
		}
	}

	return b.String()
}
