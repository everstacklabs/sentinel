package openai

import (
	"context"
	"log/slog"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/htmlutil"
)

const openAIPricingURL = "https://openai.com/api/pricing/"

// discoverFromDocs scrapes the OpenAI pricing page for cost data.
// Returns partial models (cost only) that can supplement API discovery.
func (o *OpenAI) discoverFromDocs(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	doc, err := htmlutil.Fetch(ctx, openAIPricingURL)
	if err != nil {
		return nil, err
	}

	// Try multiple table selectors — OpenAI's pricing page structure may vary.
	selectors := []string{
		"table",
		".pricing-table table",
		"[data-testid='pricing-table'] table",
	}

	var models []adapter.DiscoveredModel
	for _, sel := range selectors {
		rows := htmlutil.TableRows(doc, sel)
		if len(rows) == 0 {
			continue
		}

		for _, row := range rows {
			m := parsePricingRow(row)
			if m != nil {
				models = append(models, *m)
			}
		}
		if len(models) > 0 {
			break
		}
	}

	if len(models) == 0 {
		slog.Warn("openai docs scraping: no pricing data found (page may be JS-rendered)")
	} else {
		slog.Info("openai docs scraping complete", "models_with_pricing", len(models))
	}

	return models, nil
}

// parsePricingRow attempts to extract a model with cost data from a table row.
func parsePricingRow(row map[string]string) *adapter.DiscoveredModel {
	// Try common column name patterns.
	name := firstNonEmpty(row, "model", "name", "model name")
	if name == "" {
		return nil
	}

	inputStr := firstNonEmpty(row, "input", "input price", "input cost", "prompt")
	outputStr := firstNonEmpty(row, "output", "output price", "output cost", "completion")

	inputCost, okIn := htmlutil.ParsePriceDollars(inputStr)
	outputCost, okOut := htmlutil.ParsePriceDollars(outputStr)

	if !okIn && !okOut {
		return nil
	}

	return &adapter.DiscoveredModel{
		Name: name,
		Cost: &adapter.Cost{
			InputPer1K:  inputCost,
			OutputPer1K: outputCost,
		},
		DiscoveredBy: adapter.SourceDocs,
	}
}

func firstNonEmpty(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := m[k]; v != "" {
			return v
		}
	}
	return ""
}
