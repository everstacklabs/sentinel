package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/everstacklabs/sentinel/internal/diff"
)

// Verdict represents the judge's decision for a model.
type Verdict string

const (
	VerdictApprove Verdict = "approve"
	VerdictFlag    Verdict = "flag"
	VerdictReject  Verdict = "reject"
)

// OnRejectBehavior controls what happens when models are rejected.
type OnRejectBehavior string

const (
	OnRejectDraft   OnRejectBehavior = "draft"   // Mark PR as draft, include all models
	OnRejectExclude OnRejectBehavior = "exclude" // Remove rejected models from changeset
)

// ModelVerdict is the judge's assessment of a single model.
type ModelVerdict struct {
	ModelName  string   `json:"model_name"`
	Verdict    Verdict  `json:"verdict"`
	Confidence float64  `json:"confidence"`
	Concerns   []string `json:"concerns"`
	Reasoning  string   `json:"reasoning"`
}

// Result holds the complete judge evaluation.
type Result struct {
	Verdicts []ModelVerdict `json:"verdicts"`
}

// HasRejections reports whether any model was rejected.
func (r *Result) HasRejections() bool {
	for _, v := range r.Verdicts {
		if v.Verdict == VerdictReject {
			return true
		}
	}
	return false
}

// HasFlags reports whether any model was flagged.
func (r *Result) HasFlags() bool {
	for _, v := range r.Verdicts {
		if v.Verdict == VerdictFlag {
			return true
		}
	}
	return false
}

// RejectedNames returns the names of rejected models.
func (r *Result) RejectedNames() []string {
	var names []string
	for _, v := range r.Verdicts {
		if v.Verdict == VerdictReject {
			names = append(names, v.ModelName)
		}
	}
	return names
}

// LLMResponse is the raw response from an LLM provider.
type LLMResponse struct {
	Content string
}

// LLMClient abstracts LLM API calls for testability.
type LLMClient interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (*LLMResponse, error)
}

// Judge evaluates changesets using an LLM.
type Judge struct {
	client   LLMClient
	model    string
	disabled bool
}

// New creates a new Judge. If disabled is true, Evaluate returns nil.
func New(client LLMClient, model string, disabled bool) *Judge {
	return &Judge{
		client:   client,
		model:    model,
		disabled: disabled,
	}
}

// Evaluate sends the changeset to the LLM for review.
// Returns nil when the judge is disabled.
func (j *Judge) Evaluate(ctx context.Context, cs *diff.ChangeSet) (*Result, error) {
	if j.disabled {
		return nil, nil
	}

	if !cs.HasChanges() {
		return nil, nil
	}

	systemPrompt := buildSystemPrompt()
	userPrompt := buildUserPrompt(cs)

	resp, err := j.client.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	result, err := parseResponse(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parsing LLM response: %w", err)
	}

	return result, nil
}

// ApplyToChangeSet applies the judge result to the changeset.
// Returns forceDraft=true when behavior is "draft" and there are rejections.
// When behavior is "exclude", rejected models are removed from the changeset.
func ApplyToChangeSet(cs *diff.ChangeSet, result *Result, behavior OnRejectBehavior) (forceDraft bool) {
	if result == nil {
		return false
	}

	if !result.HasRejections() && !result.HasFlags() {
		return false
	}

	if result.HasFlags() {
		forceDraft = true
	}

	if !result.HasRejections() {
		return forceDraft
	}

	switch behavior {
	case OnRejectDraft:
		return true

	case OnRejectExclude:
		rejected := make(map[string]bool)
		for _, name := range result.RejectedNames() {
			rejected[name] = true
		}

		filtered := cs.New[:0]
		for _, m := range cs.New {
			if !rejected[m.Name] {
				filtered = append(filtered, m)
			}
		}
		cs.New = filtered

		filteredUpdates := cs.Updated[:0]
		for _, u := range cs.Updated {
			if !rejected[u.Name] {
				filteredUpdates = append(filteredUpdates, u)
			}
		}
		cs.Updated = filteredUpdates

		slog.Info("judge excluded models", "count", len(rejected))
		return false

	default:
		return true
	}
}

// parseResponse extracts the JSON verdict array from the LLM response text.
func parseResponse(content string) (*Result, error) {
	jsonStr, err := extractJSON(content)
	if err != nil {
		return nil, err
	}

	var result Result
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("unmarshaling judge response: %w", err)
	}

	// Validate verdicts
	for i, v := range result.Verdicts {
		switch v.Verdict {
		case VerdictApprove, VerdictFlag, VerdictReject:
			// valid
		default:
			return nil, fmt.Errorf("invalid verdict %q for model %s", v.Verdict, v.ModelName)
		}
		if v.Confidence < 0 || v.Confidence > 1 {
			result.Verdicts[i].Confidence = clamp(v.Confidence, 0, 1)
		}
	}

	return &result, nil
}

// extractJSON finds and returns the JSON object from text that may be
// wrapped in markdown code fences or surrounded by other text.
func extractJSON(s string) (string, error) {
	s = strings.TrimSpace(s)

	// Try to parse as-is first
	if isValidJSON(s) {
		return s, nil
	}

	// Strip markdown code fences
	if idx := strings.Index(s, "```json"); idx != -1 {
		start := idx + len("```json")
		end := strings.Index(s[start:], "```")
		if end != -1 {
			candidate := strings.TrimSpace(s[start : start+end])
			if isValidJSON(candidate) {
				return candidate, nil
			}
		}
	}
	if idx := strings.Index(s, "```"); idx != -1 {
		start := idx + len("```")
		end := strings.Index(s[start:], "```")
		if end != -1 {
			candidate := strings.TrimSpace(s[start : start+end])
			if isValidJSON(candidate) {
				return candidate, nil
			}
		}
	}

	// Find first { and last }
	first := strings.Index(s, "{")
	last := strings.LastIndex(s, "}")
	if first != -1 && last > first {
		candidate := s[first : last+1]
		if isValidJSON(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no valid JSON found in response")
}

func isValidJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
