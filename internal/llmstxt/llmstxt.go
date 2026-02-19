package llmstxt

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

// Fetch performs an HTTP GET and returns the raw text body.
// Uses the same User-Agent as htmlutil.Fetch for consistency.
func Fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Sentinel/1.0; +https://github.com/everstacklabs/sentinel)")
	req.Header.Set("Accept", "text/plain")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching %s: status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading body from %s: %w", url, err)
	}

	return string(body), nil
}

// ExtractModelIDs extracts unique model IDs from raw text using the given regex patterns.
// Each pattern should contain exactly one capturing group for the model ID.
// If a pattern has no capturing group, the full match is used.
func ExtractModelIDs(content string, patterns []*regexp.Regexp) []string {
	seen := make(map[string]struct{})
	var ids []string

	for _, pat := range patterns {
		matches := pat.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			var id string
			if len(match) > 1 {
				id = match[1]
			} else {
				id = match[0]
			}
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}

	return ids
}
