package togetherai

import (
	"context"
	"log/slog"
	"regexp"
	"strings"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/llmstxt"
)

const togetheraiLLMsTxtURL = "https://docs.together.ai/llms-full.txt"

var togetheraiModelRe = regexp.MustCompile(`([\w-]+/[\w.-]+)`)

// discoverFromDocs fetches the Together AI llms-full.txt and extracts model IDs.
func (t *TogetherAI) discoverFromDocs(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	content, err := llmstxt.Fetch(ctx, togetheraiLLMsTxtURL)
	if err != nil {
		return nil, err
	}

	ids := llmstxt.ExtractModelIDs(content, []*regexp.Regexp{togetheraiModelRe})

	var models []adapter.DiscoveredModel
	for _, id := range ids {
		if shouldSkipDocsModel(id) {
			continue
		}
		models = append(models, adapter.DiscoveredModel{
			Name:         id,
			DiscoveredBy: adapter.SourceDocs,
		})
	}

	slog.Info("togetherai llms.txt discovery complete", "models_from_llmstxt", len(models))
	return models, nil
}

// shouldSkipDocsModel filters out non-chat models from llms.txt results.
func shouldSkipDocsModel(id string) bool {
	lower := strings.ToLower(id)
	skipPatterns := []string{"embed", "rerank", "whisper", "tts", "image", "stable-diffusion", "flux", "dall-e"}
	for _, p := range skipPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
