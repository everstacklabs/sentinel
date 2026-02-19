package fireworks

import (
	"context"
	"log/slog"
	"regexp"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/llmstxt"
)

const fireworksLLMsTxtURL = "https://docs.fireworks.ai/llms-full.txt"

var fireworksModelRe = regexp.MustCompile(`(accounts/fireworks/models/[\w.-]+)`)

// discoverFromDocs fetches the Fireworks llms-full.txt and extracts model IDs.
func (f *Fireworks) discoverFromDocs(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	content, err := llmstxt.Fetch(ctx, fireworksLLMsTxtURL)
	if err != nil {
		return nil, err
	}

	ids := llmstxt.ExtractModelIDs(content, []*regexp.Regexp{fireworksModelRe})

	var models []adapter.DiscoveredModel
	for _, id := range ids {
		if shouldSkip(id) {
			continue
		}
		models = append(models, adapter.DiscoveredModel{
			Name:         id,
			DiscoveredBy: adapter.SourceDocs,
		})
	}

	slog.Info("fireworks llms.txt discovery complete", "models_from_llmstxt", len(models))
	return models, nil
}
