//go:build integration

package openai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/httpclient"
)

func TestOpenAIAPIIntegration(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	o := &OpenAI{}
	client := httpclient.New()
	o.Configure(apiKey, "https://api.openai.com/v1", client)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	models, err := o.Discover(ctx, adapter.DiscoverOptions{
		Sources: []adapter.SourceType{adapter.SourceAPI},
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("expected at least some models from OpenAI API")
	}

	// Check that well-known models are included
	modelNames := make(map[string]bool)
	for _, m := range models {
		modelNames[m.Name] = true
	}

	knownModels := []string{"gpt-4o", "gpt-3.5-turbo"}
	for _, name := range knownModels {
		if !modelNames[name] {
			t.Errorf("expected %q in discovered models", name)
		}
	}

	// Check that filtered models are excluded
	for name := range modelNames {
		if len(name) > 3 && name[:3] == "ft:" {
			t.Errorf("fine-tuned model %q should be filtered", name)
		}
		if len(name) > 6 && name[:6] == "dall-e" {
			t.Errorf("dall-e model %q should be filtered", name)
		}
	}

	// Verify discovered models have required fields
	for _, m := range models {
		if m.Name == "" {
			t.Error("model with empty name")
		}
		if m.DisplayName == "" {
			t.Errorf("model %q has empty display_name", m.Name)
		}
		if m.Family == "" {
			t.Errorf("model %q has empty family", m.Name)
		}
		if len(m.Capabilities) == 0 {
			t.Errorf("model %q has no capabilities", m.Name)
		}
		if m.Limits.MaxTokens == 0 {
			t.Errorf("model %q has zero max_tokens", m.Name)
		}
	}
}
