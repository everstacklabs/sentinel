package google

import (
	"testing"
)

func TestShouldSkip(t *testing.T) {
	genMethods := []string{"generateContent", "streamGenerateContent"}

	tests := []struct {
		id      string
		methods []string
		skip    bool
	}{
		// Should skip — versioned snapshots
		{"gemini-1.5-flash-001", genMethods, true},
		{"gemini-2.0-flash-001", genMethods, true},
		{"gemini-1.5-pro-002", genMethods, true},

		// Should skip — legacy models
		{"chat-bison-001", genMethods, true},
		{"text-bison-001", genMethods, true},
		{"embedding-001", genMethods, true},
		{"aqa", genMethods, true},

		// Should skip — no generateContent support
		{"some-model", []string{"embedContent"}, true},

		// Should NOT skip — base aliases
		{"gemini-2.0-flash", genMethods, false},
		{"gemini-1.5-flash", genMethods, false},
		{"gemini-1.5-pro", genMethods, false},
		{"gemini-2.0-flash-lite", genMethods, false},
		{"gemma-3-27b-it", genMethods, false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := shouldSkip(tt.id, tt.methods)
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
		{"gemini-2.0-flash", "gemini-2"},
		{"gemini-2.0-flash-lite", "gemini-2"},
		{"gemini-2.5-pro", "gemini-2"},
		{"gemini-1.5-flash", "gemini-1.5"},
		{"gemini-1.5-pro", "gemini-1.5"},
		{"gemini-1.0-pro", "gemini-1.0"},
		{"gemini-pro", "gemini-1.0"},
		{"gemma-3-27b-it", "gemma"},
		{"gemma-2-9b-it", "gemma"},
		{"unknown-model", "google-other"},
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
	genMethods := []string{"generateContent", "streamGenerateContent"}
	genOnly := []string{"generateContent"}

	tests := []struct {
		id       string
		methods  []string
		wantCaps []string
	}{
		{"gemini-2.0-flash", genMethods, []string{"chat", "function_calling", "vision", "streaming"}},
		{"gemini-1.5-pro", genOnly, []string{"chat", "function_calling", "vision"}},
		{"gemma-3-27b-it", genMethods, []string{"chat", "streaming"}},
		{"gemini-2.0-flash-thinking", genMethods, []string{"chat", "function_calling", "vision", "streaming", "thinking"}},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := inferCapabilities(tt.id, tt.methods)
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
	t.Run("gemini model", func(t *testing.T) {
		m := inferModalities("gemini-2.0-flash")
		if len(m.Input) != 4 {
			t.Errorf("input = %v, want [text, image, video, audio]", m.Input)
		}
		if len(m.Output) != 1 || m.Output[0] != "text" {
			t.Errorf("output = %v, want [text]", m.Output)
		}
	})

	t.Run("gemma model", func(t *testing.T) {
		m := inferModalities("gemma-3-27b-it")
		if len(m.Input) != 1 || m.Input[0] != "text" {
			t.Errorf("input = %v, want [text]", m.Input)
		}
		if len(m.Output) != 1 || m.Output[0] != "text" {
			t.Errorf("output = %v, want [text]", m.Output)
		}
	})
}
