package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/everstacklabs/sentinel/internal/cache"
	"golang.org/x/time/rate"
)

// Client is an HTTP client with caching, rate limiting, and conditional fetch.
type Client struct {
	http    *http.Client
	cache   *cache.FileCache
	limiter *rate.Limiter
	noCache bool
}

// Option configures the Client.
type Option func(*Client)

// WithCache enables file-based caching.
func WithCache(c *cache.FileCache) Option {
	return func(cl *Client) { cl.cache = c }
}

// WithRateLimit sets requests per second.
func WithRateLimit(rps float64) Option {
	return func(cl *Client) {
		cl.limiter = rate.NewLimiter(rate.Limit(rps), 1)
	}
}

// WithNoCache disables caching.
func WithNoCache() Option {
	return func(cl *Client) { cl.noCache = true }
}

// New creates a new HTTP client.
func New(opts ...Option) *Client {
	c := &Client{
		http: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Response wraps an HTTP response body and metadata.
type Response struct {
	Body       []byte
	StatusCode int
	FromCache  bool
}

// Get performs an HTTP GET with optional caching and conditional fetch.
func (c *Client) Get(ctx context.Context, url string, headers map[string]string) (*Response, error) {
	// Rate limit
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}
	}

	// Check cache
	var staleEntry *cache.Entry
	if c.cache != nil && !c.noCache {
		entry, fresh := c.cache.Get(url)
		if fresh {
			return &Response{Body: entry.Body, StatusCode: entry.StatusCode, FromCache: true}, nil
		}
		staleEntry = entry // Use for conditional fetch
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Conditional fetch headers
	if staleEntry != nil {
		if staleEntry.ETag != "" {
			req.Header.Set("If-None-Match", staleEntry.ETag)
		}
		if staleEntry.LastMod != "" {
			req.Header.Set("If-Modified-Since", staleEntry.LastMod)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	// Not modified — refresh cache TTL
	if resp.StatusCode == http.StatusNotModified && staleEntry != nil {
		if c.cache != nil {
			_ = c.cache.Set(url, staleEntry)
		}
		return &Response{Body: staleEntry.Body, StatusCode: staleEntry.StatusCode, FromCache: true}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP GET %s: status %d: %s", url, resp.StatusCode, string(body))
	}

	// Store in cache
	if c.cache != nil && !c.noCache {
		_ = c.cache.Set(url, &cache.Entry{
			Body:       body,
			ETag:       resp.Header.Get("ETag"),
			LastMod:    resp.Header.Get("Last-Modified"),
			StatusCode: resp.StatusCode,
		})
	}

	return &Response{Body: body, StatusCode: resp.StatusCode}, nil
}
