package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/everstacklabs/sentinel/internal/catalog"
	"github.com/everstacklabs/sentinel/internal/diff"
)

// mockClient implements LLMClient for testing.
type mockClient struct {
	response string
	err      error
}

func (m *mockClient) Complete(_ context.Context, _, _ string) (*LLMResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &LLMResponse{Content: m.response}, nil
}

func makeChangeSet() *diff.ChangeSet {
	return &diff.ChangeSet{
		Provider: "openai",
		New: []diff.ModelChange{
			{
				Name: "gpt-5",
				Model: &catalog.Model{
					Name:         "gpt-5",
					DisplayName:  "GPT-5",
					Family:       "gpt-5",
					Status:       "generally_available",
					Capabilities: []string{"chat", "function_calling"},
					Modalities:   catalog.Modalities{Input: []string{"text"}, Output: []string{"text"}},
					Limits:       catalog.Limits{MaxTokens: 128000, MaxCompletionTokens: 16384},
					Cost:         &catalog.Cost{InputPer1K: 0.005, OutputPer1K: 0.015},
				},
			},
		},
		Updated: []diff.ModelUpdate{
			{
				Name: "gpt-4o",
				Model: &catalog.Model{
					Name:         "gpt-4o",
					DisplayName:  "GPT-4o",
					Family:       "gpt-4o",
					Status:       "generally_available",
					Capabilities: []string{"chat", "function_calling", "vision"},
					Modalities:   catalog.Modalities{Input: []string{"text", "image"}, Output: []string{"text"}},
					Limits:       catalog.Limits{MaxTokens: 128000, MaxCompletionTokens: 16384},
					Cost:         &catalog.Cost{InputPer1K: 0.0025, OutputPer1K: 0.01},
				},
				Changes: []catalog.FieldChange{
					{Field: "cost.input_per_1k", OldValue: 0.005, NewValue: 0.0025},
				},
			},
		},
	}
}

func allApprovedResponse() string {
	r := Result{
		Verdicts: []ModelVerdict{
			{ModelName: "gpt-5", Verdict: VerdictApprove, Confidence: 0.95, Reasoning: "looks correct"},
			{ModelName: "gpt-4o", Verdict: VerdictApprove, Confidence: 0.9, Reasoning: "price drop is plausible"},
		},
	}
	b, _ := json.Marshal(r)
	return string(b)
}

func withRejectionResponse() string {
	r := Result{
		Verdicts: []ModelVerdict{
			{ModelName: "gpt-5", Verdict: VerdictReject, Confidence: 0.85, Concerns: []string{"suspicious pricing"}, Reasoning: "price too low for frontier model"},
			{ModelName: "gpt-4o", Verdict: VerdictApprove, Confidence: 0.9, Reasoning: "looks correct"},
		},
	}
	b, _ := json.Marshal(r)
	return string(b)
}

// --- Evaluate tests ---

func TestEvaluate_AllApproved(t *testing.T) {
	client := &mockClient{response: allApprovedResponse()}
	j := New(client, "test-model", false)

	result, err := j.Evaluate(context.Background(), makeChangeSet())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Verdicts) != 2 {
		t.Fatalf("expected 2 verdicts, got %d", len(result.Verdicts))
	}
	if result.HasRejections() {
		t.Error("expected no rejections")
	}
	if result.HasFlags() {
		t.Error("expected no flags")
	}
}

func TestEvaluate_WithRejection(t *testing.T) {
	client := &mockClient{response: withRejectionResponse()}
	j := New(client, "test-model", false)

	result, err := j.Evaluate(context.Background(), makeChangeSet())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasRejections() {
		t.Error("expected rejections")
	}
	names := result.RejectedNames()
	if len(names) != 1 || names[0] != "gpt-5" {
		t.Errorf("expected rejected [gpt-5], got %v", names)
	}
}

func TestEvaluate_Disabled(t *testing.T) {
	client := &mockClient{response: "should not be called"}
	j := New(client, "test-model", true)

	result, err := j.Evaluate(context.Background(), makeChangeSet())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when disabled")
	}
}

func TestEvaluate_NoChanges(t *testing.T) {
	client := &mockClient{response: "should not be called"}
	j := New(client, "test-model", false)

	cs := &diff.ChangeSet{Provider: "openai"}
	result, err := j.Evaluate(context.Background(), cs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for empty changeset")
	}
}

func TestEvaluate_ClientError(t *testing.T) {
	client := &mockClient{err: fmt.Errorf("API timeout")}
	j := New(client, "test-model", false)

	_, err := j.Evaluate(context.Background(), makeChangeSet())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "LLM call failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// --- parseResponse tests ---

func TestParseResponse_MarkdownFencedJSON(t *testing.T) {
	raw := "Here's my analysis:\n```json\n" + allApprovedResponse() + "\n```\nDone."
	result, err := parseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Verdicts) != 2 {
		t.Fatalf("expected 2 verdicts, got %d", len(result.Verdicts))
	}
}

func TestParseResponse_PlainJSON(t *testing.T) {
	result, err := parseResponse(allApprovedResponse())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Verdicts) != 2 {
		t.Fatalf("expected 2 verdicts, got %d", len(result.Verdicts))
	}
}

func TestParseResponse_InvalidVerdict(t *testing.T) {
	r := Result{
		Verdicts: []ModelVerdict{
			{ModelName: "test", Verdict: "maybe", Confidence: 0.5},
		},
	}
	b, _ := json.Marshal(r)
	_, err := parseResponse(string(b))
	if err == nil {
		t.Fatal("expected error for invalid verdict")
	}
	if !strings.Contains(err.Error(), "invalid verdict") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- ApplyToChangeSet tests ---

func TestApplyToChangeSet_DraftMode(t *testing.T) {
	cs := makeChangeSet()
	result := &Result{
		Verdicts: []ModelVerdict{
			{ModelName: "gpt-5", Verdict: VerdictReject, Confidence: 0.85},
			{ModelName: "gpt-4o", Verdict: VerdictApprove, Confidence: 0.9},
		},
	}

	forceDraft := ApplyToChangeSet(cs, result, OnRejectDraft)
	if !forceDraft {
		t.Error("expected forceDraft=true in draft mode with rejections")
	}
	// Draft mode should NOT mutate the changeset
	if len(cs.New) != 1 {
		t.Errorf("expected 1 new model (unchanged), got %d", len(cs.New))
	}
	if len(cs.Updated) != 1 {
		t.Errorf("expected 1 updated model (unchanged), got %d", len(cs.Updated))
	}
}

func TestApplyToChangeSet_ExcludeMode(t *testing.T) {
	cs := makeChangeSet()
	result := &Result{
		Verdicts: []ModelVerdict{
			{ModelName: "gpt-5", Verdict: VerdictReject, Confidence: 0.85},
			{ModelName: "gpt-4o", Verdict: VerdictApprove, Confidence: 0.9},
		},
	}

	forceDraft := ApplyToChangeSet(cs, result, OnRejectExclude)
	if forceDraft {
		t.Error("expected forceDraft=false in exclude mode")
	}
	if len(cs.New) != 0 {
		t.Errorf("expected 0 new models after exclusion, got %d", len(cs.New))
	}
	if len(cs.Updated) != 1 {
		t.Errorf("expected 1 updated model (not rejected), got %d", len(cs.Updated))
	}
}

func TestApplyToChangeSet_NilResult(t *testing.T) {
	cs := makeChangeSet()
	forceDraft := ApplyToChangeSet(cs, nil, OnRejectDraft)
	if forceDraft {
		t.Error("expected forceDraft=false for nil result")
	}
}

func TestApplyToChangeSet_AllApproved(t *testing.T) {
	cs := makeChangeSet()
	result := &Result{
		Verdicts: []ModelVerdict{
			{ModelName: "gpt-5", Verdict: VerdictApprove, Confidence: 0.95},
			{ModelName: "gpt-4o", Verdict: VerdictApprove, Confidence: 0.9},
		},
	}
	forceDraft := ApplyToChangeSet(cs, result, OnRejectExclude)
	if forceDraft {
		t.Error("expected forceDraft=false when all approved")
	}
}

func TestApplyToChangeSet_FlagForceDraft(t *testing.T) {
	cs := makeChangeSet()
	result := &Result{
		Verdicts: []ModelVerdict{
			{ModelName: "gpt-5", Verdict: VerdictFlag, Confidence: 0.7},
			{ModelName: "gpt-4o", Verdict: VerdictApprove, Confidence: 0.9},
		},
	}
	forceDraft := ApplyToChangeSet(cs, result, OnRejectDraft)
	if !forceDraft {
		t.Error("expected forceDraft=true when models are flagged")
	}
}

// --- RenderSection tests ---

func TestRenderSection_AllApproved(t *testing.T) {
	result := &Result{
		Verdicts: []ModelVerdict{
			{ModelName: "gpt-5", Verdict: VerdictApprove, Confidence: 0.95},
		},
	}
	section := RenderSection(result)
	if section != "" {
		t.Errorf("expected empty string for all approved, got %q", section)
	}
}

func TestRenderSection_Nil(t *testing.T) {
	section := RenderSection(nil)
	if section != "" {
		t.Errorf("expected empty string for nil, got %q", section)
	}
}

func TestRenderSection_WithFlags(t *testing.T) {
	result := &Result{
		Verdicts: []ModelVerdict{
			{ModelName: "gpt-5", Verdict: VerdictFlag, Confidence: 0.7, Concerns: []string{"odd pricing"}, Reasoning: "needs review"},
			{ModelName: "gpt-4o", Verdict: VerdictApprove, Confidence: 0.9},
		},
	}
	section := RenderSection(result)
	if !strings.Contains(section, "LLM Judge Review") {
		t.Error("expected section header")
	}
	if !strings.Contains(section, "Flagged Models") {
		t.Error("expected flagged table")
	}
	if !strings.Contains(section, "gpt-5") {
		t.Error("expected gpt-5 in flagged table")
	}
	if !strings.Contains(section, "odd pricing") {
		t.Error("expected concern in table")
	}
}

func TestRenderSection_WithRejections(t *testing.T) {
	result := &Result{
		Verdicts: []ModelVerdict{
			{ModelName: "bad-model", Verdict: VerdictReject, Confidence: 0.9, Concerns: []string{"wrong capabilities"}, Reasoning: "embedding with chat"},
		},
	}
	section := RenderSection(result)
	if !strings.Contains(section, "Rejected Models") {
		t.Error("expected rejected table")
	}
	if !strings.Contains(section, "bad-model") {
		t.Error("expected bad-model in rejected table")
	}
}

// --- extractJSON tests ---

func TestExtractJSON_PlainJSON(t *testing.T) {
	input := `{"verdicts": []}`
	result, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("expected %q, got %q", input, result)
	}
}

func TestExtractJSON_MarkdownFence(t *testing.T) {
	input := "```json\n{\"verdicts\": []}\n```"
	result, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"verdicts": []}` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExtractJSON_PlainFence(t *testing.T) {
	input := "```\n{\"verdicts\": []}\n```"
	result, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"verdicts": []}` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExtractJSON_SurroundingText(t *testing.T) {
	input := "Here is my analysis:\n{\"verdicts\": []}\nThank you."
	result, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"verdicts": []}` {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	input := "This is not JSON at all."
	_, err := extractJSON(input)
	if err == nil {
		t.Fatal("expected error for non-JSON input")
	}
}

func TestExtractJSON_WhitespaceWrapped(t *testing.T) {
	input := "\n\n  {\"verdicts\": []}  \n\n"
	result, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"verdicts": []}` {
		t.Errorf("unexpected result: %q", result)
	}
}

// --- buildUserPrompt test ---

func TestBuildUserPrompt_IncludesModels(t *testing.T) {
	cs := makeChangeSet()
	prompt := buildUserPrompt(cs)

	if !strings.Contains(prompt, "gpt-5") {
		t.Error("expected gpt-5 in prompt")
	}
	if !strings.Contains(prompt, "gpt-4o") {
		t.Error("expected gpt-4o in prompt")
	}
	if !strings.Contains(prompt, "New Models") {
		t.Error("expected New Models section")
	}
	if !strings.Contains(prompt, "Updated Models") {
		t.Error("expected Updated Models section")
	}
}
