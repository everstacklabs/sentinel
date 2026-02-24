package google

import "testing"

func TestParseGeminiModelDocText(t *testing.T) {
	text := `
Model ID

gemini-3.1-pro-preview

Token limits

Maximum input tokens: 1,048,576
Maximum output tokens: 65,536
`

	modelID, limits := parseGeminiModelDocText(text)
	if modelID != "gemini-3.1-pro-preview" {
		t.Fatalf("modelID = %q, want %q", modelID, "gemini-3.1-pro-preview")
	}
	if limits.MaxTokens != 1048576 {
		t.Fatalf("MaxTokens = %d, want %d", limits.MaxTokens, 1048576)
	}
	if limits.MaxCompletionTokens != 65536 {
		t.Fatalf("MaxCompletionTokens = %d, want %d", limits.MaxCompletionTokens, 65536)
	}
}

func TestParseTokenLimitMissing(t *testing.T) {
	val := parseTokenLimit("no tokens here", "Maximum input tokens")
	if val != 0 {
		t.Fatalf("expected 0, got %d", val)
	}
}
