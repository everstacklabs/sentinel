package google

import (
	"context"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/htmlutil"
)

const googleModelsURL = "https://cloud.google.com/vertex-ai/generative-ai/docs/learn/models"
const googleGeminiPathPrefix = "/vertex-ai/generative-ai/docs/models/gemini/"

var googleModelIDRe = regexp.MustCompile(`(?i)gemini-[a-z0-9.-]+`)

// discoverFromDocs scrapes the Google models page and detail pages for Gemini model IDs.
func (g *Google) discoverFromDocs(ctx context.Context) ([]adapter.DiscoveredModel, error) {
	doc, err := htmlutil.Fetch(ctx, googleModelsURL)
	if err != nil {
		return nil, err
	}

	links := extractGeminiLinks(doc)
	if len(links) == 0 {
		slog.Warn("google docs scraping: no Gemini model links found")
		return nil, nil
	}

	var models []adapter.DiscoveredModel
	for _, link := range links {
		page, err := htmlutil.Fetch(ctx, link)
		if err != nil {
			slog.Warn("google docs scraping: failed to fetch model detail", "url", link, "error", err)
			continue
		}

		m := parseGeminiModelDoc(page)
		if m != nil {
			models = append(models, *m)
		}
	}

	if len(models) == 0 {
		slog.Warn("google docs scraping: no models discovered")
	} else {
		slog.Info("google docs scraping complete", "models_from_docs", len(models))
	}

	return models, nil
}

func extractGeminiLinks(doc *goquery.Document) []string {
	seen := make(map[string]struct{})
	var links []string

	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok || href == "" {
			return
		}
		if !strings.Contains(href, googleGeminiPathPrefix) {
			return
		}
		url := href
		if strings.HasPrefix(href, "/") {
			url = "https://cloud.google.com" + href
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		links = append(links, url)
	})

	return links
}

func parseGeminiModelDoc(doc *goquery.Document) *adapter.DiscoveredModel {
	text := doc.Text()
	modelID, limits := parseGeminiModelDocText(text)
	if modelID == "" || limits.MaxTokens == 0 {
		return nil
	}

	status := "stable"
	if strings.Contains(modelID, "preview") {
		status = "preview"
	}

	return &adapter.DiscoveredModel{
		Name:         modelID,
		DisplayName:  inferDisplayName(modelID),
		Family:       inferFamily(modelID),
		Status:       status,
		Capabilities: inferCapabilities(modelID, []string{"generateContent"}),
		Modalities:   inferModalities(modelID),
		Limits:       limits,
		DiscoveredBy: adapter.SourceDocs,
	}
}

func parseGeminiModelDocText(text string) (string, adapter.Limits) {
	modelID := findFirstModelID(text)
	inputTokens := parseTokenLimit(text, "Maximum input tokens")
	outputTokens := parseTokenLimit(text, "Maximum output tokens")

	return modelID, adapter.Limits{
		MaxTokens:           inputTokens,
		MaxCompletionTokens: outputTokens,
	}
}

func findFirstModelID(text string) string {
	match := googleModelIDRe.FindString(text)
	if match == "" {
		return ""
	}
	return strings.ToLower(match)
}

func parseTokenLimit(text, label string) int {
	pattern := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(label) + `\s*:\s*([\d,]+)`)
	matches := pattern.FindStringSubmatch(text)
	if len(matches) < 2 {
		return 0
	}

	num := strings.ReplaceAll(matches[1], ",", "")
	val, err := strconv.Atoi(num)
	if err != nil {
		return 0
	}
	return val
}
