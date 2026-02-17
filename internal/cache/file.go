package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Entry represents a cached HTTP response.
type Entry struct {
	Body       []byte    `json:"body"`
	ETag       string    `json:"etag,omitempty"`
	LastMod    string    `json:"last_modified,omitempty"`
	StatusCode int       `json:"status_code"`
	CachedAt   time.Time `json:"cached_at"`
}

// FileCache provides TTL-based file caching for HTTP responses.
type FileCache struct {
	dir string
	ttl time.Duration
}

// New creates a new file cache.
func New(dir string, ttl time.Duration) (*FileCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}
	return &FileCache{dir: dir, ttl: ttl}, nil
}

// Get retrieves a cached entry if it exists and hasn't expired.
func (c *FileCache) Get(key string) (*Entry, bool) {
	path := c.path(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		os.Remove(path)
		return nil, false
	}

	if time.Since(entry.CachedAt) > c.ttl {
		// Expired but return for conditional fetch (ETag/If-Modified-Since)
		return &entry, false
	}

	return &entry, true
}

// Set stores an entry in the cache.
func (c *FileCache) Set(key string, entry *Entry) error {
	entry.CachedAt = time.Now()
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling cache entry: %w", err)
	}
	path := c.path(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c *FileCache) path(key string) string {
	h := sha256.Sum256([]byte(key))
	return filepath.Join(c.dir, hex.EncodeToString(h[:]))
}
