package perplexity

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/htmlutil"
)

const perplexityModelsURL = "https://docs.perplexity.ai/getting-started/models"

func (p *Perplexity) discoverFromDocs(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	doc, err := htmlutil.Fetch(ctx, perplexityModelsURL)
	if err != nil {
		return nil, err
	}

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
			m := parseModelRow(row)
			if m != nil {
				models = append(models, *m)
			}
		}
		if len(models) > 0 {
			break
		}
	}

	if len(models) == 0 {
		slog.Warn("perplexity docs scraping: no model data found (page may be JS-rendered)")
	} else {
		slog.Info("perplexity docs scraping complete", "models", len(models))
	}

	return models, nil
}

func parseModelRow(row map[string]string) *adapter.DiscoveredModel {
	name := firstNonEmpty(row, "model", "model name", "name", "api model name")
	if name == "" {
		return nil
	}

	m := &adapter.DiscoveredModel{
		Name:         name,
		DisplayName:  inferDisplayName(name),
		Family:       inferFamily(name),
		Status:       "stable",
		Capabilities: inferCapabilities(name),
		Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		DiscoveredBy: adapter.SourceDocs,
	}

	// Try to extract context window from docs.
	contextStr := firstNonEmpty(row, "context length", "context window", "context", "max tokens")
	if tokens := parseTokenCount(contextStr); tokens > 0 {
		m.Limits = adapter.Limits{MaxTokens: tokens}
	}

	// Try to extract cost.
	inputStr := firstNonEmpty(row, "input", "input price", "price per input token")
	outputStr := firstNonEmpty(row, "output", "output price", "price per output token")
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

func inferFamily(id string) string {
	lower := strings.ToLower(id)
	switch {
	case strings.Contains(lower, "sonar-deep-research"):
		return "sonar-deep-research"
	case strings.Contains(lower, "sonar-reasoning"):
		return "sonar-reasoning"
	case strings.Contains(lower, "sonar-pro"):
		return "sonar-pro"
	case strings.Contains(lower, "sonar"):
		return "sonar"
	default:
		return "perplexity-other"
	}
}

func inferDisplayName(id string) string {
	parts := strings.Split(id, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func inferCapabilities(id string) []string {
	caps := []string{"chat", "streaming"}
	lower := strings.ToLower(id)
	if strings.Contains(lower, "reasoning") {
		caps = append(caps, "reasoning")
	}
	// All Sonar models have search/citation capability.
	if strings.Contains(lower, "sonar") {
		caps = append(caps, "search")
	}
	return caps
}

// parseTokenCount tries to extract a token count from strings like "127k", "128,000", "200000".
func parseTokenCount(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")

	if strings.HasSuffix(s, "k") {
		s = strings.TrimSuffix(s, "k")
		var n float64
		if _, err := fmt.Sscanf(s, "%f", &n); err == nil {
			return int(n * 1000)
		}
	}

	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
		return n
	}
	return 0
}

func firstNonEmpty(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := m[k]; v != "" {
			return v
		}
	}
	return ""
}
