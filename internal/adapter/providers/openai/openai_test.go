package openai

import (
	"testing"
)

func TestShouldSkip(t *testing.T) {
	o := &OpenAI{}
	tests := []struct {
		id   string
		skip bool
	}{
		// Should skip
		{"ft:gpt-4o:my-org:custom:id", true},
		{"ft:gpt-3.5-turbo:acme:suffix:id", true},
		{"gpt-4-0613", true},
		{"gpt-4-1106-preview", true},
		{"gpt-3.5-turbo-0125", true},
		{"gpt-4o-2024-05-13", true},
		{"gpt-5-2025-08-07", true},
		{"gpt-4o-mini-2024-07-18", true},
		{"dall-e-3", true},
		{"tts-1", true},
		{"tts-1-hd", true},
		{"whisper-1", true},
		{"text-moderation-latest", true},
		{"babbage-002", true},
		{"davinci-002", true},
		{"curie-001", true},
		{"ada-002", true},

		// Should NOT skip
		{"gpt-4o", false},
		{"gpt-4o-mini", false},
		{"gpt-4-turbo", false},
		{"gpt-3.5-turbo", false},
		{"o3", false},
		{"o3-mini", false},
		{"o1", false},
		{"text-embedding-3-small", false},
		{"text-embedding-3-large", false},
		{"gpt-5", false},
		{"gpt-5.1-codex", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := o.shouldSkip(tt.id)
			if got != tt.skip {
				t.Errorf("shouldSkip(%q) = %v, want %v", tt.id, got, tt.skip)
			}
		})
	}
}

func TestIsDateSnapshot(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"gpt-4-0613", true},
		{"gpt-4-1106-preview", true},
		{"gpt-3.5-turbo-0125", true},
		{"gpt-4-turbo-20240409", true},
		{"gpt-4o-2024-05-13", true},
		{"gpt-5-2025-08-07", true},
		{"gpt-4o-mini-2024-07-18", true},
		{"o1-2024-12-17", true},
		{"gpt-4o", false},
		{"gpt-4-turbo", false},
		{"o3-mini", false},
		{"text-embedding-3-small", false},
		{"gpt-3.5-turbo", false},
		{"gpt-5", false},
		{"gpt-5.1-codex", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := isDateSnapshot(tt.id)
			if got != tt.want {
				t.Errorf("isDateSnapshot(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestIsDateLike(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"0613", true},
		{"1106", true},
		{"0125", true},
		{"20240409", true},
		{"mini", false},
		{"turbo", false},
		{"3", false},
		{"12345", false}, // 5 digits
		{"abc", false},
		{"06a3", false},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got := isDateLike(tt.s)
			if got != tt.want {
				t.Errorf("isDateLike(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestInferFamily(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"gpt-4o", "gpt-4"},
		{"gpt-4o-mini", "gpt-4"},
		{"gpt-4-turbo", "gpt-4"},
		{"gpt-4.1", "gpt-4"},
		{"gpt-3.5-turbo", "gpt-3.5"},
		{"gpt-5", "gpt-5"},
		{"gpt-5.1-codex", "gpt-5"},
		{"gpt-5.2-codex", "gpt-5"},
		{"gpt-5.3-codex", "gpt-5"},
		{"o3", "o-series"},
		{"o3-mini", "o-series"},
		{"o4-mini", "o-series"},
		{"o4-mini-deep-research", "o-series"},
		{"o1", "o-series"},
		{"o1-mini", "o-series"},
		{"text-embedding-3-small", "embedding"},
		{"text-embedding-3-large", "embedding"},
		{"unknown-model", "other"},
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

func TestInferDisplayName(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"gpt-4o", "GPT-4o"},
		{"gpt-4o-mini", "GPT-4o Mini"},
		{"gpt-4-turbo", "GPT-4 Turbo"},
		{"gpt-5.1-codex", "GPT-5.1 Codex"},
		{"o3-mini", "O3 Mini"},
		{"unknown-model", "Unknown Model"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := inferDisplayName(tt.id)
			if got != tt.want {
				t.Errorf("inferDisplayName(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

func TestInferCapabilities(t *testing.T) {
	tests := []struct {
		id       string
		wantCaps []string
	}{
		{"text-embedding-3-small", []string{"embeddings"}},
		{"text-embedding-3-large", []string{"embeddings"}},
		{"gpt-4o", []string{"chat", "function_calling", "vision"}},
		{"gpt-4-turbo", []string{"chat", "function_calling", "vision"}},
		{"gpt-5", []string{"chat", "function_calling", "vision"}},
		{"gpt-4.1", []string{"chat", "function_calling", "vision"}},
		{"gpt-3.5-turbo", []string{"chat", "function_calling"}},
		{"o3", []string{"chat", "function_calling"}},
		{"gpt-3.5-turbo-instruct", []string{"chat"}},
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
	t.Run("embedding model", func(t *testing.T) {
		caps := []string{"embeddings"}
		m := inferModalities("text-embedding-3-small", caps)
		if len(m.Input) != 1 || m.Input[0] != "text" {
			t.Errorf("embedding input = %v, want [text]", m.Input)
		}
		if len(m.Output) != 1 || m.Output[0] != "embedding" {
			t.Errorf("embedding output = %v, want [embedding]", m.Output)
		}
	})

	t.Run("vision model", func(t *testing.T) {
		caps := []string{"chat", "function_calling", "vision"}
		m := inferModalities("gpt-4o", caps)
		if len(m.Input) != 2 || m.Input[0] != "text" || m.Input[1] != "image" {
			t.Errorf("vision input = %v, want [text, image]", m.Input)
		}
		if len(m.Output) != 1 || m.Output[0] != "text" {
			t.Errorf("vision output = %v, want [text]", m.Output)
		}
	})

	t.Run("text-only model", func(t *testing.T) {
		caps := []string{"chat", "function_calling"}
		m := inferModalities("gpt-3.5-turbo", caps)
		if len(m.Input) != 1 || m.Input[0] != "text" {
			t.Errorf("text input = %v, want [text]", m.Input)
		}
		if len(m.Output) != 1 || m.Output[0] != "text" {
			t.Errorf("text output = %v, want [text]", m.Output)
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
		{"gpt-5", "gpt-5", 128000, 16384},
		{"gpt-4o", "gpt-4", 128000, 16384},
		{"gpt-4o-mini", "gpt-4", 128000, 16384},
		{"gpt-3.5-turbo", "gpt-3.5", 16385, 4096},
		{"o3", "o-series", 200000, 100000},
		{"text-embedding-3-small", "embedding", 8191, 0},
		{"unknown", "other", 128000, 0},
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
