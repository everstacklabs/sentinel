package pipeline

import (
	"strings"
	"testing"

	"github.com/everstacklabs/sentinel/internal/catalog"
	"github.com/everstacklabs/sentinel/internal/diff"
)

func TestNormalizeMergeMethod(t *testing.T) {
	tests := map[string]string{
		"":        "SQUASH",
		"squash":  "SQUASH",
		"SQUASH":  "SQUASH",
		"merge":   "MERGE",
		" rebase": "REBASE",
		"unknown": "SQUASH",
	}

	for input, want := range tests {
		if got := normalizeMergeMethod(input); got != want {
			t.Fatalf("normalizeMergeMethod(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRenderBatchPRBody(t *testing.T) {
	body := renderBatchPRBody([]*diff.ChangeSet{
		{
			Provider: "openai",
			New: []diff.ModelChange{
				{
					Name: "gpt-5-mini",
					Model: &catalog.Model{
						Name:   "gpt-5-mini",
						Family: "gpt-5",
						Status: "stable",
						Limits: catalog.Limits{MaxTokens: 128000},
					},
				},
			},
			Updated: []diff.ModelUpdate{
				{
					Name: "gpt-4o",
					Changes: []catalog.FieldChange{
						{Field: "limits.max_tokens", OldValue: 128000, NewValue: 200000},
					},
				},
			},
		},
	}, nil)

	for _, want := range []string{
		"## Automated Model Catalog Sync",
		"`openai` | 1 | 1 | 0",
		"`gpt-5-mini`",
		"`gpt-4o`",
		"limits.max_tokens: 128000 -> 200000",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("batch PR body missing %q:\n%s", want, body)
		}
	}
}
