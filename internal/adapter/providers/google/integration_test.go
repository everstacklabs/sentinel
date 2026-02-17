//go:build integration

package google

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/httpclient"
)

func TestGoogleAPIIntegration(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set")
	}

	g := &Google{}
	client := httpclient.New()
	g.Configure(apiKey, "https://generativelanguage.googleapis.com/v1beta", client)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	models, err := g.Discover(ctx, adapter.DiscoverOptions{
		Sources: []adapter.SourceType{adapter.SourceAPI},
	})
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(models) == 0 {
		t.Fatal("expected at least some models from Google API")
	}

	// Check that well-known models are included
	modelNames := make(map[string]bool)
	for _, m := range models {
		modelNames[m.Name] = true
	}

	knownModels := []string{"gemini-2.0-flash", "gemini-1.5-pro"}
	for _, name := range knownModels {
		if !modelNames[name] {
			t.Errorf("expected %q in discovered models", name)
		}
	}

	// Check that versioned snapshots are excluded
	for name := range modelNames {
		if datedSnapshotRe.MatchString(name) {
			t.Errorf("versioned snapshot %q should be filtered", name)
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
