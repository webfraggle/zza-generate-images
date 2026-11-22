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

// Cache is a file-based cache: rendered images (PNG and GIF) and, in a second
// instance with its own folder, ZZA packages. Cache keys are SHA-256 hashes of
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
func (c *Cache) Get(key string) ([]byte, bool) { return c.GetExt(key, "png") }

// GetExt returns the cached bytes for key with file extension ext
// (e.g. "png", "gif"), or (nil, false) on miss or invalid ext.
// RLock prevents reads from racing with cleanup deletions.
func (c *Cache) GetExt(key, ext string) ([]byte, bool) {
	if !validExt(ext) {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	path := c.filePath(key, ext)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

// Set writes PNG data to the cache under key.
func (c *Cache) Set(key string, data []byte) error { return c.SetExt(key, "png", data) }

// SetExt writes data to the cache under key with file extension ext.
// Lock serialises writes with concurrent cleanup.
func (c *Cache) SetExt(key, ext string, data []byte) error {
	if !validExt(ext) {
		return fmt.Errorf("cache: invalid extension %q", ext)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	path := c.filePath(key, ext)
	// Write to a temporary file and rename it into place: a full disk or a
	// kill mid-write must not leave a truncated file that later reads as a hit.
	tmp, err := os.CreateTemp(c.dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("cache: writing %q: %w", path, err)
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmpName, 0o644)
	}
	if werr == nil {
		werr = os.Rename(tmpName, path)
	}
	if werr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("cache: writing %q: %w", path, werr)
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
			// The cache is flat. Subfolders belong to other caches with their
			// own limits (the zza package cache) and are left alone.
			if filepath.Clean(path) != absDir {
				return filepath.SkipDir
			}
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

// CacheStats holds a snapshot of the current cache state.
type CacheStats struct {
	FileCount  int
	TotalBytes int64
	Dir        string
}

// Stats returns a snapshot of the current cache contents.
func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var totalBytes int64
	var count int
	root := filepath.Clean(c.dir)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root {
				return filepath.SkipDir // other caches' subfolders are not counted
			}
			return nil
		}
		if info, infoErr := d.Info(); infoErr == nil {
			totalBytes += info.Size()
			count++
		}
		return nil
	})
	return CacheStats{FileCount: count, TotalBytes: totalBytes, Dir: c.dir}
}

// Flush removes all cached files from the cache directory.
func (c *Cache) Flush() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return fmt.Errorf("cache: flush: reading dir: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			_ = os.Remove(filepath.Join(c.dir, e.Name()))
		}
	}
	return nil
}

func (c *Cache) filePath(key, ext string) string {
	// key is a 64-char hex string and ext passed validExt — safe as filename.
	return filepath.Join(c.dir, key+"."+ext)
}

// validExt reports whether ext is a short lower-case alphanumeric extension.
func validExt(ext string) bool {
	if ext == "" || len(ext) > 8 {
		return false
	}
	for _, r := range ext {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
