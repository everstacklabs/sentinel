package anthropic

import (
	"context"
	"log/slog"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/htmlutil"
)

const anthropicModelsURL = "https://docs.anthropic.com/en/docs/about-claude/models"

// discoverFromDocs scrapes the Anthropic models documentation page.
// Returns partial models with metadata that can supplement API discovery.
func (a *Anthropic) discoverFromDocs(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	doc, err := htmlutil.Fetch(ctx, anthropicModelsURL)
	if err != nil {
		return nil, err
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
		slog.Warn("anthropic docs scraping: no model data found (page may be JS-rendered)")
	} else {
		slog.Info("anthropic docs scraping complete", "models_from_docs", len(models))
	}

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
