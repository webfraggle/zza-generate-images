package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/version"
)

// A new build must not serve images cached by an older renderer.
func TestRenderCacheKey_ChangesWithVersion(t *testing.T) {
	srv := newFlapServer(t)
	old := version.Version
	t.Cleanup(func() { version.Version = old })

	version.Version = "v1.0.0"
	k1 := srv.renderCacheKey("t", []byte(`{}`))
	version.Version = "v1.0.1"
	k2 := srv.renderCacheKey("t", []byte(`{}`))
	if k1 == k2 {
		t.Error("cache key did not change with the build version")
	}
}

// Replacing an asset (image, font, default.json) must invalidate the cache,
// not only edits to template.yaml.
func TestRenderCacheKey_ChangesWithAssetFile(t *testing.T) {
	srv := newFlapServer(t)
	asset := filepath.Join(srv.templatesDir, "t", "bg.png")
	if err := os.WriteFile(asset, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(asset, past, past); err != nil {
		t.Fatal(err)
	}
	k1 := srv.renderCacheKey("t", []byte(`{}`))

	if err := os.Chtimes(asset, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	k2 := srv.renderCacheKey("t", []byte(`{}`))
	if k1 == k2 {
		t.Error("cache key did not change after an asset file was modified")
	}

	if err := os.WriteFile(asset, []byte("longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(asset, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if k3 := srv.renderCacheKey("t", []byte(`{}`)); k3 == k2 {
		t.Error("cache key did not change after an asset file changed size")
	}
}

func TestRenderCacheKey_StableAndBodySensitive(t *testing.T) {
	srv := newFlapServer(t)
	a := srv.renderCacheKey("t", []byte(`{}`))
	if b := srv.renderCacheKey("t", []byte(`{}`)); a != b {
		t.Error("cache key is not deterministic")
	}
	if c := srv.renderCacheKey("t", []byte(`{"x":1}`)); a == c {
		t.Error("cache key ignores the request body")
	}
}

// End to end: an entry written under an older build is not served.
func TestRender_StaleEntryFromOldVersionIsMiss(t *testing.T) {
	srv := newFlapServer(t)
	old := version.Version
	t.Cleanup(func() { version.Version = old })

	version.Version = "v1.0.0"
	if err := srv.cache.Set(srv.renderCacheKey("t", []byte(`{}`)), []byte("STALE")); err != nil {
		t.Fatal(err)
	}
	version.Version = "v1.0.1"
	rr := doRender(srv, "/t/render", `{}`, nil)
	if rr.Code != 200 || rr.Header().Get("X-Cache") != "MISS" || rr.Body.String() == "STALE" {
		t.Errorf("got %d, X-Cache %q; want a fresh render (MISS)", rr.Code, rr.Header().Get("X-Cache"))
	}
}
