package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Cache is a file-based PNG cache. Cache keys are SHA-256 hashes of
// (template name + request body). A background goroutine evicts stale
// and oversized entries periodically.
type Cache struct {
	dir       string
	maxAge    time.Duration
	maxBytes  int64 // maximum total directory size in bytes
	mu        sync.RWMutex
}

// NewCache creates a Cache that stores files in dir and starts no goroutines.
// Call StartCleanup to enable background eviction.
func NewCache(dir string, maxAge time.Duration, maxSizeMB int64) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cache: creating directory %q: %w", dir, err)
	}
	return &Cache{
		dir:      dir,
		maxAge:   maxAge,
		maxBytes: maxSizeMB * 1024 * 1024,
	}, nil
}

// Key returns a deterministic hex cache key for (templateName, body).
func (c *Cache) Key(templateName string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(templateName))
	h.Write([]byte{0}) // separator
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// Get returns the cached PNG bytes for key, or (nil, false) on miss.
// RLock prevents reads from racing with cleanup deletions.
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	path := c.filePath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

// Set writes data to the cache under key.
// Lock serialises writes with concurrent cleanup.
func (c *Cache) Set(key string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	path := c.filePath(key)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("cache: writing %q: %w", path, err)
	}
	return nil
}

// StartCleanup runs the eviction loop in a background goroutine until ctx is done.
func (c *Cache) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.cleanup(); err != nil {
					log.Printf("cache cleanup error: %v", err)
				}
			}
		}
	}()
}

// cleanup removes files older than maxAge, then removes the oldest files
// until the total directory size is below maxBytes.
func (c *Cache) cleanup() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Resolve the canonical cache directory once to guard against symlink escapes.
	absDir, err := filepath.Abs(c.dir)
	if err != nil {
		return fmt.Errorf("cache cleanup: resolving cache dir: %w", err)
	}
	absDirPrefix := absDir + string(filepath.Separator)

	type entry struct {
		path    string
		size    int64
		modTime time.Time
	}

	var entries []entry
	var totalBytes int64
	cutoff := time.Now().Add(-c.maxAge)

	// Walk absDir (not c.dir) so that path comparison with absDirPrefix is consistent.
	err = filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		// Guard against symlinks that escape the cache directory.
		cleanPath := filepath.Clean(path)
		if cleanPath != absDir && !strings.HasPrefix(cleanPath, absDirPrefix) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		// Delete files older than maxAge immediately.
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
			return nil
		}
		entries = append(entries, entry{path: path, size: info.Size(), modTime: info.ModTime()})
		totalBytes += info.Size()
		return nil
	})
	if err != nil {
		return fmt.Errorf("cache cleanup walk: %w", err)
	}

	if totalBytes <= c.maxBytes {
		return nil
	}

	// Sort oldest first; delete until under limit.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].modTime.Before(entries[j].modTime)
	})
	for _, e := range entries {
		if totalBytes <= c.maxBytes {
			break
		}
		if removeErr := os.Remove(e.path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			log.Printf("cache: removing %q: %v", e.path, removeErr)
		} else {
			// File was removed (or already gone) — subtract its size from the total.
			totalBytes -= e.size
		}
	}
	return nil
}

func (c *Cache) filePath(key string) string {
	// key is a 64-char hex string — safe to use directly as filename.
	return filepath.Join(c.dir, key+".png")
}
