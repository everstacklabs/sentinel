//go:build integration

package anthropic

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/httpclient"
)

func TestAnthropicAPIIntegration(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	a := &Anthropic{}
	client := httpclient.New()
	a.Configure(apiKey, "https://api.anthropic.com/v1", client)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	models, err := a.Discover(ctx, adapter.DiscoverOptions{
		Sources: []adapter.SourceType{adapter.SourceAPI},
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("expected at least some models from Anthropic API")
	}

	// Check that well-known models are included
	modelNames := make(map[string]bool)
	for _, m := range models {
		modelNames[m.Name] = true
	}

	knownModels := []string{"claude-sonnet-4-0", "claude-haiku-4-0"}
	for _, name := range knownModels {
		if !modelNames[name] {
			t.Errorf("expected %q in discovered models", name)
		}
	}

	// Check that dated snapshots are excluded
	for name := range modelNames {
		if datedSnapshotRe.MatchString(name) {
			t.Errorf("dated snapshot %q should be filtered", name)
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
