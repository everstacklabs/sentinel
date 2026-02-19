package mistral

import (
	"context"
	"log/slog"
	"regexp"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/llmstxt"
)

const mistralLLMsTxtURL = "https://docs.mistral.ai/llms-full.txt"

var mistralModelRe = regexp.MustCompile(`(mistral-[\w-]+)`)

// discoverFromDocs fetches the Mistral llms-full.txt and extracts model IDs.
func (m *Mistral) discoverFromDocs(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	content, err := llmstxt.Fetch(ctx, mistralLLMsTxtURL)
	if err != nil {
		return nil, err
	}

	ids := llmstxt.ExtractModelIDs(content, []*regexp.Regexp{mistralModelRe})

	var models []adapter.DiscoveredModel
	for _, id := range ids {
		models = append(models, adapter.DiscoveredModel{
			Name:         id,
			DiscoveredBy: adapter.SourceDocs,
		})
	}

	slog.Info("mistral llms.txt discovery complete", "models_from_llmstxt", len(models))
	return models, nil
}
