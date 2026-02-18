package htmlutil

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// TableRows extracts table rows as header→value maps from the first table
// matching the given CSS selector. The first row (or <thead>) is used as headers.
func TableRows(doc *goquery.Document, selector string) []map[string]string {
	var rows []map[string]string

	table := doc.Find(selector).First()
	if table.Length() == 0 {
		return nil
	}

	// Extract headers from <thead> or first <tr>.
	var headers []string
	thead := table.Find("thead tr").First()
	if thead.Length() > 0 {
		thead.Find("th").Each(func(_ int, s *goquery.Selection) {
			headers = append(headers, normalizeHeader(s.Text()))
		})
	}

	bodyRows := table.Find("tbody tr")
	if bodyRows.Length() == 0 {
		// Fallback: all <tr> elements, first is header.
		allRows := table.Find("tr")
		if allRows.Length() < 2 {
			return nil
		}
		if len(headers) == 0 {
			allRows.First().Find("th, td").Each(func(_ int, s *goquery.Selection) {
				headers = append(headers, normalizeHeader(s.Text()))
			})
		}
		bodyRows = allRows.Slice(1, allRows.Length())
	}

	if len(headers) == 0 {
		return nil
	}

	bodyRows.Each(func(_ int, row *goquery.Selection) {
		m := make(map[string]string, len(headers))
		row.Find("td").Each(func(i int, cell *goquery.Selection) {
			if i < len(headers) {
				m[headers[i]] = strings.TrimSpace(cell.Text())
			}
		})
		if len(m) > 0 {
			rows = append(rows, m)
		}
	})

	return rows
}

// TextOf returns the trimmed text of the first element matching the selector.
func TextOf(doc *goquery.Document, selector string) string {
	return strings.TrimSpace(doc.Find(selector).First().Text())
}

// priceRe matches patterns like "$0.150", "$0.150 / 1M tokens", "$15.00 / 1M".
var priceRe = regexp.MustCompile(`\$\s*([\d,.]+)`)

// ParsePriceDollars parses a price string like "$0.150 / 1M tokens" and
// converts it to a per-1K token cost (float64). Returns (value, true) on
// success or (0, false) if parsing fails.
func ParsePriceDollars(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "—" || s == "-" || s == "N/A" {
		return 0, false
	}

	matches := priceRe.FindStringSubmatch(s)
	if len(matches) < 2 {
		return 0, false
	}

	numStr := strings.ReplaceAll(matches[1], ",", "")
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, false
	}

	// Detect if the price is per 1M tokens and convert to per 1K.
	lower := strings.ToLower(s)
	if strings.Contains(lower, "1m") || strings.Contains(lower, "million") {
		val = val / 1000.0 // per-1M → per-1K
	}

	return val, true
}

func normalizeHeader(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
