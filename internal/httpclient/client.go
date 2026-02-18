package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/everstacklabs/sentinel/internal/cache"
	"golang.org/x/time/rate"
)

// Client is an HTTP client with caching, per-host rate limiting, and retry.
type Client struct {
	http         *http.Client
	cache        *cache.FileCache
	noCache      bool
	defaultRPS   float64
	maxRetries   int
	baseBackoff  time.Duration
	hostLimiters map[string]*rate.Limiter
	mu           sync.RWMutex
}

// Option configures the Client.
type Option func(*Client)

// WithCache enables file-based caching.
func WithCache(c *cache.FileCache) Option {
	return func(cl *Client) { cl.cache = c }
}

// WithRateLimit sets the default requests-per-second for each host.
func WithRateLimit(rps float64) Option {
	return func(cl *Client) { cl.defaultRPS = rps }
}

// WithNoCache disables caching.
func WithNoCache() Option {
	return func(cl *Client) { cl.noCache = true }
}

// WithMaxRetries sets the maximum number of retries for retryable errors.
func WithMaxRetries(n int) Option {
	return func(cl *Client) { cl.maxRetries = n }
}

// WithBaseBackoff sets the base backoff duration for exponential retry.
func WithBaseBackoff(d time.Duration) Option {
	return func(cl *Client) { cl.baseBackoff = d }
}

// New creates a new HTTP client.
func New(opts ...Option) *Client {
	c := &Client{
		http:         &http.Client{Timeout: 30 * time.Second},
		defaultRPS:   5,
		maxRetries:   3,
		baseBackoff:  500 * time.Millisecond,
		hostLimiters: make(map[string]*rate.Limiter),
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

// retryableError indicates a transient HTTP error that can be retried.
type retryableError struct {
	statusCode int
	retryAfter time.Duration
	err        error
}

func (e *retryableError) Error() string {
	return fmt.Sprintf("retryable HTTP %d: %v", e.statusCode, e.err)
}

func (e *retryableError) Unwrap() error { return e.err }

// limiterForHost returns the per-host rate limiter, creating one if needed.
func (c *Client) limiterForHost(host string) *rate.Limiter {
	c.mu.RLock()
	lim, ok := c.hostLimiters[host]
	c.mu.RUnlock()
	if ok {
		return lim
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Double-check after acquiring write lock.
	if lim, ok = c.hostLimiters[host]; ok {
		return lim
	}
	lim = rate.NewLimiter(rate.Limit(c.defaultRPS), 1)
	c.hostLimiters[host] = lim
	return lim
}

// Get performs an HTTP GET with per-host rate limiting, caching, and retry.
func (c *Client) Get(ctx context.Context, rawURL string, headers map[string]string) (*Response, error) {
	// Check cache first (before rate-limiting or retrying).
	var staleEntry *cache.Entry
	if c.cache != nil && !c.noCache {
		entry, fresh := c.cache.Get(rawURL)
		if fresh {
			return &Response{Body: entry.Body, StatusCode: entry.StatusCode, FromCache: true}, nil
		}
		staleEntry = entry
	}

	// Per-host rate limit.
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %w", err)
	}
	lim := c.limiterForHost(parsed.Host)

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			slog.Debug("retrying request", "url", rawURL, "attempt", attempt)
		}

		if err := lim.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait: %w", err)
		}

		resp, err := c.doRequest(ctx, rawURL, headers, staleEntry)
		if err == nil {
			return resp, nil
		}

		var retryErr *retryableError
		if !errors.As(err, &retryErr) {
			return nil, err // Non-retryable error.
		}

		lastErr = retryErr

		// Determine backoff.
		backoff := retryErr.retryAfter
		if backoff == 0 {
			backoff = c.baseBackoff * time.Duration(math.Pow(2, float64(attempt)))
		}

		slog.Warn("retryable error, backing off",
			"url", rawURL,
			"status", retryErr.statusCode,
			"backoff", backoff,
			"attempt", attempt+1,
			"max_retries", c.maxRetries)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// doRequest performs a single HTTP GET request.
func (c *Client) doRequest(ctx context.Context, rawURL string, headers map[string]string, staleEntry *cache.Entry) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Conditional fetch headers.
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
		return nil, fmt.Errorf("HTTP GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	// Not modified — refresh cache TTL.
	if resp.StatusCode == http.StatusNotModified && staleEntry != nil {
		if c.cache != nil {
			_ = c.cache.Set(rawURL, staleEntry)
		}
		return &Response{Body: staleEntry.Body, StatusCode: staleEntry.StatusCode, FromCache: true}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	// 429 Too Many Requests — retryable.
	if resp.StatusCode == http.StatusTooManyRequests {
		ra := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, &retryableError{
			statusCode: resp.StatusCode,
			retryAfter: ra,
			err:        fmt.Errorf("HTTP GET %s: status 429: %s", rawURL, string(body)),
		}
	}

	// 5xx Server Error — retryable.
	if resp.StatusCode >= 500 {
		return nil, &retryableError{
			statusCode: resp.StatusCode,
			err:        fmt.Errorf("HTTP GET %s: status %d: %s", rawURL, resp.StatusCode, string(body)),
		}
	}

	// Other 4xx — non-retryable.
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP GET %s: status %d: %s", rawURL, resp.StatusCode, string(body))
	}

	// Store in cache.
	if c.cache != nil && !c.noCache {
		_ = c.cache.Set(rawURL, &cache.Entry{
			Body:       body,
			ETag:       resp.Header.Get("ETag"),
			LastMod:    resp.Header.Get("Last-Modified"),
			StatusCode: resp.StatusCode,
		})
	}

	return &Response{Body: body, StatusCode: resp.StatusCode}, nil
}

// parseRetryAfter parses the Retry-After header value.
// Handles both integer seconds ("120") and HTTP-date ("Fri, 31 Dec 1999 23:59:59 GMT").
func parseRetryAfter(s string) time.Duration {
	if s == "" {
		return 0
	}

	// Try integer seconds first.
	if secs, err := strconv.Atoi(s); err == nil {
		return time.Duration(secs) * time.Second
	}

	// Try HTTP-date.
	if t, err := http.ParseTime(s); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}

	return 0
}
