package anthropic

import (
	"context"
	"log/slog"
	"regexp"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/htmlutil"
	"github.com/everstacklabs/sentinel/internal/llmstxt"
)

const anthropicModelsURL = "https://platform.claude.com/docs/en/about-claude/models/overview"
const anthropicLLMsTxtURL = "https://platform.claude.com/docs/en/about-claude/models/llms-full.txt"

var claudeModelRe = regexp.MustCompile(`(claude-[\w.-]+)`)

// discoverFromDocs scrapes the Anthropic models documentation page.
// Falls back to llms-full.txt if the HTML page is JS-rendered and yields no models.
func (a *Anthropic) discoverFromDocs(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	doc, err := htmlutil.Fetch(ctx, anthropicModelsURL)
	if err != nil {
		slog.Warn("anthropic docs HTML fetch failed, trying llms.txt fallback", "error", err)
		return a.discoverFromLLMsTxt(ctx)
	}

	// Try multiple table selectors.
	selectors := []string{
		"table",
		".markdown table",
		"article table",
	}

	var models []adapter.DiscoveredModel
	for _, sel := range selectors {
		rows := htmlutil.TableRows(doc, sel)
		if len(rows) == 0 {
			continue
		}

		for _, row := range rows {
			m := parseDocsRow(row)
			if m != nil {
				models = append(models, *m)
			}
		}
		if len(models) > 0 {
			break
		}
	}

	if len(models) == 0 {
		slog.Warn("anthropic docs scraping: no model data found (page may be JS-rendered), trying llms.txt fallback")
		return a.discoverFromLLMsTxt(ctx)
	}

	slog.Info("anthropic docs scraping complete", "models_from_docs", len(models))
	return models, nil
}

// discoverFromLLMsTxt fetches the llms-full.txt and extracts claude model IDs.
func (a *Anthropic) discoverFromLLMsTxt(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	content, err := llmstxt.Fetch(ctx, anthropicLLMsTxtURL)
	if err != nil {
		return nil, err
	}

	ids := llmstxt.ExtractModelIDs(content, []*regexp.Regexp{claudeModelRe})

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

	slog.Info("anthropic llms.txt discovery complete", "models_from_llmstxt", len(models))
	return models, nil
}

// parseDocsRow attempts to extract a model from a docs table row.
func parseDocsRow(row map[string]string) *adapter.DiscoveredModel {
	name := firstNonEmpty(row, "model", "model name", "api model name", "name")
	if name == "" {
		return nil
	}

	m := &adapter.DiscoveredModel{
		Name:         name,
		DiscoveredBy: adapter.SourceDocs,
	}

	// Try to extract cost data if present.
	inputStr := firstNonEmpty(row, "input", "input price", "input cost")
	outputStr := firstNonEmpty(row, "output", "output price", "output cost")
	inputCost, okIn := htmlutil.ParsePriceDollars(inputStr)
	outputCost, okOut := htmlutil.ParsePriceDollars(outputStr)
	if okIn || okOut {
		m.Cost = &adapter.Cost{
			InputPer1K:  inputCost,
			OutputPer1K: outputCost,
		}
	}

	return m
}

func firstNonEmpty(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := m[k]; v != "" {
			return v
		}
	}
	return ""
}
