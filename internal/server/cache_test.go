package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCache_SetGet(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCache(dir, time.Hour, 100)
	if err != nil {
		t.Fatal(err)
	}

	key := c.Key("tmpl", []byte(`{"a":1}`))
	data := []byte("fake png bytes")

	if _, hit := c.Get(key); hit {
		t.Fatal("expected cache miss before Set")
	}
	if err := c.Set(key, data); err != nil {
		t.Fatal(err)
	}
	got, hit := c.Get(key)
	if !hit {
		t.Fatal("expected cache hit after Set")
	}
	if string(got) != string(data) {
		t.Errorf("got %q, want %q", got, data)
	}
}

func TestCache_KeyDiffersForDiffInputs(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCache(dir, time.Hour, 100)
	k1 := c.Key("a", []byte("x"))
	k2 := c.Key("b", []byte("x"))
	k3 := c.Key("a", []byte("y"))
	if k1 == k2 || k1 == k3 || k2 == k3 {
		t.Error("keys should differ for different inputs")
	}
}

func TestCache_CleanupAge(t *testing.T) {
	dir := t.TempDir()
	// maxAge = 1 nanosecond → all files should be deleted immediately
	c, _ := NewCache(dir, time.Nanosecond, 1000)
	key := c.Key("t", []byte("{}"))
	_ = c.Set(key, []byte("data"))

	time.Sleep(5 * time.Millisecond)
	if err := c.cleanup(); err != nil {
		t.Fatal(err)
	}

	if _, hit := c.Get(key); hit {
		t.Error("expected file to be evicted by age")
	}
}

func TestCache_CleanupSize(t *testing.T) {
	dir := t.TempDir()
	// maxSize = 1 byte → all files exceed limit after first write
	c, _ := NewCache(dir, 24*time.Hour, 0) // 0 MB = 0 bytes limit
	// Write two files
	k1 := c.Key("t", []byte("a"))
	k2 := c.Key("t", []byte("b"))
	_ = c.Set(k1, []byte("AAAA"))
	time.Sleep(5 * time.Millisecond) // ensure different mtime
	_ = c.Set(k2, []byte("BBBB"))

	if err := c.cleanup(); err != nil {
		t.Fatal(err)
	}

	// With 0-byte limit both files should be deleted.
	entries, _ := filepath.Glob(filepath.Join(dir, "*.png"))
	if len(entries) != 0 {
		t.Errorf("expected all files deleted, got %d", len(entries))
	}
}

func TestCache_DirCreated(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "dir")
	_, err := NewCache(dir, time.Hour, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}
