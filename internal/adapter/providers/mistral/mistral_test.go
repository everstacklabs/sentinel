package mistral

import (
	"testing"
)

func TestShouldSkip(t *testing.T) {
	tests := []struct {
		name string
		am   apiModel
		skip bool
	}{
		{"fine-tuned model", apiModel{ID: "ft:mistral-small:custom", Type: "fine-tuned"}, true},
		{"deprecated model", apiModel{ID: "mistral-old", Deprecation: strPtr("2025-01-01")}, true},
		{"embedding model", apiModel{ID: "mistral-embed"}, true},
		{"base model", apiModel{ID: "mistral-large-latest", Type: "base"}, false},
		{"codestral", apiModel{ID: "codestral-latest", Type: "base"}, false},
		{"pixtral", apiModel{ID: "pixtral-large-latest", Type: "base"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkip(tt.am)
			if got != tt.skip {
				t.Errorf("shouldSkip(%q) = %v, want %v", tt.am.ID, got, tt.skip)
			}
		})
	}
}

func TestInferFamily(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"mistral-large-latest", "mistral-large"},
		{"mistral-large-2411", "mistral-large"},
		{"mistral-medium-latest", "mistral-medium"},
		{"mistral-small-latest", "mistral-small"},
		{"mistral-tiny-latest", "mistral-tiny"},
		{"codestral-latest", "codestral"},
		{"codestral-mamba-latest", "codestral"},
		{"pixtral-large-latest", "pixtral"},
		{"pixtral-12b-2409", "pixtral"},
		{"ministral-8b-latest", "ministral"},
		{"open-mixtral-8x22b", "mixtral"},
		{"open-mistral-nemo", "mistral-nemo"},
		{"unknown-model", "mistral-other"},
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

func TestBuildCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		caps     apiModelCapabilities
		wantCaps []string
	}{
		{
			"chat with function calling",
			apiModelCapabilities{CompletionChat: true, FunctionCalling: true},
			[]string{"chat", "function_calling", "streaming"},
		},
		{
			"chat with vision",
			apiModelCapabilities{CompletionChat: true, FunctionCalling: true, Vision: true},
			[]string{"chat", "function_calling", "vision", "streaming"},
		},
		{
			"code model with FIM",
			apiModelCapabilities{CompletionChat: true, CompletionFIM: true},
			[]string{"chat", "fill_in_middle", "streaming"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCapabilities(tt.caps)
			if len(got) != len(tt.wantCaps) {
				t.Fatalf("buildCapabilities() = %v, want %v", got, tt.wantCaps)
			}
			for i, c := range got {
				if c != tt.wantCaps[i] {
					t.Errorf("buildCapabilities()[%d] = %q, want %q", i, c, tt.wantCaps[i])
				}
			}
		})
	}
}

func TestInferModalities(t *testing.T) {
	t.Run("text only", func(t *testing.T) {
		m := inferModalities(apiModelCapabilities{CompletionChat: true})
		if len(m.Input) != 1 || m.Input[0] != "text" {
			t.Errorf("input = %v, want [text]", m.Input)
		}
	})

	t.Run("with vision", func(t *testing.T) {
		m := inferModalities(apiModelCapabilities{CompletionChat: true, Vision: true})
		if len(m.Input) != 2 || m.Input[1] != "image" {
			t.Errorf("input = %v, want [text, image]", m.Input)
		}
	})
}

func TestInferMaxCompletion(t *testing.T) {
	tests := []struct {
		id            string
		contextLength int
		want          int
	}{
		{"mistral-large-latest", 128000, 16384},
		{"mistral-small-latest", 32000, 8192},
		{"mistral-tiny", 8000, 4096},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := inferMaxCompletion(tt.id, tt.contextLength)
			if got != tt.want {
				t.Errorf("inferMaxCompletion(%q, %d) = %d, want %d", tt.id, tt.contextLength, got, tt.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
