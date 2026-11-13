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

func TestCache_KeyUnchanged(t *testing.T) {
	c, _ := NewCache(t.TempDir(), time.Hour, 100)
	const want = "59b271ae1bbcb1d31d41929817f4b16fb439eb4f31520b5ad1d5ce98920a7138" // sha256("a\x00b"), fixed before GIF support
	if got := c.Key("a", []byte("b")); got != want {
		t.Errorf("Key changed: got %s, want %s", got, want)
	}
}

func TestCache_Ext(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCache(dir, time.Hour, 100)
	k := c.Key("t", []byte("x"))
	if err := c.Set(k, []byte("png")); err != nil {
		t.Fatal(err)
	}
	if err := c.SetExt(k, "gif", []byte("gif")); err != nil {
		t.Fatal(err)
	}
	if got, hit := c.Get(k); !hit || string(got) != "png" {
		t.Errorf("Get = %q, %v", got, hit)
	}
	if got, hit := c.GetExt(k, "gif"); !hit || string(got) != "gif" {
		t.Errorf("GetExt = %q, %v", got, hit)
	}
	if st := c.Stats(); st.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", st.FileCount)
	}
	if err := c.Flush(); err != nil {
		t.Fatal(err)
	}
	if _, hit := c.Get(k); hit {
		t.Error("png survived Flush")
	}
	if _, hit := c.GetExt(k, "gif"); hit {
		t.Error("gif survived Flush")
	}
}

func TestCache_Ext_CleanupByAge(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCache(dir, time.Hour, 100)
	k := c.Key("t", []byte("x"))
	_ = c.SetExt(k, "gif", []byte("gif"))
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(filepath.Join(dir, k+".gif"), old, old); err != nil {
		t.Fatal(err)
	}
	if err := c.cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, hit := c.GetExt(k, "gif"); hit {
		t.Error("old gif not removed by cleanup")
	}
}

func TestCache_Ext_RejectsBadExt(t *testing.T) {
	c, _ := NewCache(t.TempDir(), time.Hour, 100)
	if err := c.SetExt("k", "../x", []byte("a")); err == nil {
		t.Error("expected error for unsafe extension")
	}
	if _, hit := c.GetExt("k", "../x"); hit {
		t.Error("expected miss for unsafe extension")
	}
}
