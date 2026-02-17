//go:build integration

package mistral

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/httpclient"
)

func TestMistralAPIIntegration(t *testing.T) {
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set")
	}

	m := &Mistral{}
	client := httpclient.New()
	m.Configure(apiKey, "https://api.mistral.ai/v1", client)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	models, err := m.Discover(ctx, adapter.DiscoverOptions{
		Sources: []adapter.SourceType{adapter.SourceAPI},
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("expected at least some models from Mistral API")
	}

	// Check that well-known models are included
	modelNames := make(map[string]bool)
	for _, m := range models {
		modelNames[m.Name] = true
	}

	knownModels := []string{"mistral-large-latest", "mistral-small-latest"}
	for _, name := range knownModels {
		if !modelNames[name] {
			t.Errorf("expected %q in discovered models", name)
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
