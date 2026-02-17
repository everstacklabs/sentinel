package anthropic

import (
	"testing"
)

func TestShouldSkip(t *testing.T) {
	tests := []struct {
		id   string
		skip bool
	}{
		// Should skip — dated snapshots
		{"claude-sonnet-4-20250514", true},
		{"claude-3-5-sonnet-20241022", true},
		{"claude-3-5-haiku-20241022", true},
		{"claude-3-opus-20240229", true},
		{"claude-3-haiku-20240307", true},

		// Should NOT skip — base aliases
		{"claude-sonnet-4-0", false},
		{"claude-haiku-4-0", false},
		{"claude-opus-4-0", false},
		{"claude-3-5-sonnet-latest", false},
		{"claude-3-5-haiku-latest", false},
		{"claude-3-opus-latest", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := shouldSkip(tt.id)
			if got != tt.skip {
				t.Errorf("shouldSkip(%q) = %v, want %v", tt.id, got, tt.skip)
			}
		})
	}
}

func TestInferFamily(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"claude-opus-4-0", "claude-opus"},
		{"claude-3-opus-latest", "claude-opus"},
		{"claude-sonnet-4-0", "claude-sonnet"},
		{"claude-3-5-sonnet-latest", "claude-sonnet"},
		{"claude-haiku-4-0", "claude-haiku"},
		{"claude-3-5-haiku-latest", "claude-haiku"},
		{"claude-unknown", "claude"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := inferFamily(tt.id)
			if got != tt.want {
				t.Errorf("inferFamily(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestInferCapabilities(t *testing.T) {
	tests := []struct {
		id       string
		wantCaps []string
	}{
		{"claude-opus-4-0", []string{"chat", "function_calling", "vision", "streaming", "extended_thinking"}},
		{"claude-sonnet-4-0", []string{"chat", "function_calling", "vision", "streaming", "extended_thinking"}},
		{"claude-haiku-4-0", []string{"chat", "function_calling", "vision", "streaming"}},
		{"claude-3-5-haiku-latest", []string{"chat", "function_calling", "vision", "streaming"}},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := inferCapabilities(tt.id)
			if len(got) != len(tt.wantCaps) {
				t.Fatalf("inferCapabilities(%q) = %v, want %v", tt.id, got, tt.wantCaps)
			}
			for i, c := range got {
				if c != tt.wantCaps[i] {
					t.Errorf("inferCapabilities(%q)[%d] = %q, want %q", tt.id, i, c, tt.wantCaps[i])
				}
			}
		})
	}
}

func TestInferModalities(t *testing.T) {
	t.Run("claude model", func(t *testing.T) {
		m := inferModalities("claude-sonnet-4-0")
		if len(m.Input) != 2 || m.Input[0] != "text" || m.Input[1] != "image" {
			t.Errorf("input = %v, want [text, image]", m.Input)
		}
		if len(m.Output) != 1 || m.Output[0] != "text" {
			t.Errorf("output = %v, want [text]", m.Output)
		}
	})
}

func TestInferLimits(t *testing.T) {
	tests := []struct {
		id              string
		family          string
		wantMax         int
		wantCompletions int
	}{
		{"claude-opus-4-0", "claude-opus", 200000, 8192},
		{"claude-sonnet-4-0", "claude-sonnet", 200000, 8192},
		{"claude-haiku-4-0", "claude-haiku", 200000, 4096},
		{"claude-3-5-sonnet-latest", "claude-sonnet", 200000, 8192},
		{"claude-3-5-haiku-latest", "claude-haiku", 200000, 4096},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			l := inferLimits(tt.id, tt.family)
			if l.MaxTokens != tt.wantMax {
				t.Errorf("inferLimits(%q).MaxTokens = %d, want %d", tt.id, l.MaxTokens, tt.wantMax)
			}
			if l.MaxCompletionTokens != tt.wantCompletions {
				t.Errorf("inferLimits(%q).MaxCompletionTokens = %d, want %d", tt.id, l.MaxCompletionTokens, tt.wantCompletions)
			}
		})
	}
}
