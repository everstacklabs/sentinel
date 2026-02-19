package cohere

import (
	"context"
	"log/slog"
	"regexp"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/llmstxt"
)

const cohereLLMsTxtURL = "https://docs.cohere.com/llms-full.txt"

var cohereModelPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(command-[\w.-]+)`),
	regexp.MustCompile(`(embed-[\w.-]+)`),
	regexp.MustCompile(`(rerank-[\w.-]+)`),
}

// discoverFromDocs fetches the Cohere llms-full.txt and extracts model IDs.
func (c *Cohere) discoverFromDocs(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	content, err := llmstxt.Fetch(ctx, cohereLLMsTxtURL)
	if err != nil {
		return nil, err
	}

	ids := llmstxt.ExtractModelIDs(content, cohereModelPatterns)

	var models []adapter.DiscoveredModel
	for _, id := range ids {
		models = append(models, adapter.DiscoveredModel{
			Name:         id,
			DiscoveredBy: adapter.SourceDocs,
		})
	}

	slog.Info("cohere llms.txt discovery complete", "models_from_llmstxt", len(models))
	return models, nil
}
