package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/config"
	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/web"
)

// zzaTestServer returns a server over a private templates dir holding a copy
// of the compatible sbb-105-v1 and the incompatible template "klein".
func zzaTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join("..", "..", "templates", "sbb-105-v1")
	dst := filepath.Join(dir, "sbb-105-v1")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	klein := filepath.Join(dir, "klein")
	if err := os.MkdirAll(klein, 0o755); err != nil {
		t.Fatal(err)
	}
	y := "meta:\n  name: klein\n  canvas:\n    width: 160\n    height: 80\nlayers:\n" +
		"  - type: rect\n    x: 0\n    y: 0\n    width: 10\n    height: 10\n    color: \"#fff\"\n"
	if err := os.WriteFile(filepath.Join(klein, "template.yaml"), []byte(y), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := New(&config.Config{
		Port:              "8080",
		TemplatesDir:      dir,
		CacheDir:          t.TempDir(),
		CacheMaxAgeHours:  1,
		CacheMaxSizeMB:    10,
		ZZACacheMaxSizeMB: 5,
	}, web.FS)
	if err != nil {
		t.Fatal(err)
	}
	return srv, dir
}

func get(srv *Server, path string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
	return rr
}

func TestZZAExport_ServesThePackageOfTheExporter(t *testing.T) {
	srv, dir := zzaTestServer(t)
	rr := get(srv, "/sbb-105-v1/zza.zip")
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
	}
	pkg, err := devexport.Export(dir, "sbb-105-v1")
	if err != nil {
		t.Fatal(err)
	}
	want, err := pkg.ZIP()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rr.Body.Bytes(), want) {
		t.Errorf("body differs from the exporter's package (%d vs %d bytes)", rr.Body.Len(), len(want))
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := rr.Header().Get("Content-Disposition"); cd != `attachment; filename="sbb-105-v1-zza.zip"` {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if xc := rr.Header().Get("X-Cache"); xc != "MISS" {
		t.Errorf("first request X-Cache = %q", xc)
	}
}

func TestZZAExport_SecondRequestComesFromTheCache(t *testing.T) {
	srv, _ := zzaTestServer(t)
	first := get(srv, "/sbb-105-v1/zza.zip")
	second := get(srv, "/sbb-105-v1/zza.zip")
	if second.Code != http.StatusOK || second.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("second: %d, X-Cache %q", second.Code, second.Header().Get("X-Cache"))
	}
	if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Error("cached package differs")
	}
	if n := srv.zzaCache.Stats().FileCount; n != 1 {
		t.Errorf("zza cache holds %d files, want 1", n)
	}
	// The package lives in the zza cache, not in the image cache.
	if n := srv.cache.Stats().FileCount; n != 0 {
		t.Errorf("image cache counts %d files, want 0", n)
	}
}

func TestZZAExport_ChangedTemplateIsExportedAgain(t *testing.T) {
	srv, dir := zzaTestServer(t)
	first := get(srv, "/sbb-105-v1/zza.zip")

	p := filepath.Join(dir, "sbb-105-v1", "template.yaml")
	y, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	y = append(y, []byte("\n  - type: rect\n    x: 1\n    y: 2\n    width: 3\n    height: 4\n    color: \"#123456\"\n")...)
	if err := os.WriteFile(p, y, 0o644); err != nil {
		t.Fatal(err)
	}
	// Same size is impossible here, but make the mtime differ for sure.
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(p, later, later); err != nil {
		t.Fatal(err)
	}

	second := get(srv, "/sbb-105-v1/zza.zip")
	if second.Code != http.StatusOK {
		t.Fatalf("got %d: %s", second.Code, second.Body.String())
	}
	if second.Header().Get("X-Cache") != "MISS" {
		t.Errorf("changed template served from cache")
	}
	if bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Error("package did not change with the template")
	}
	// The new package replaced the old one: one cache file per template.
	if n := srv.zzaCache.Stats().FileCount; n != 1 {
		t.Errorf("zza cache holds %d files after a template change, want 1", n)
	}
	if third := get(srv, "/sbb-105-v1/zza.zip"); third.Header().Get("X-Cache") != "HIT" ||
		!bytes.Equal(third.Body.Bytes(), second.Body.Bytes()) {
		t.Errorf("third request: X-Cache %q or different bytes", third.Header().Get("X-Cache"))
	}
}

// A template that is not compatible is checked once per template state;
// further requests answer from memory without taking an export slot.
func TestZZAExport_IncompatibleResultIsRemembered(t *testing.T) {
	srv, dir := zzaTestServer(t)
	if rr := get(srv, "/klein/zza.zip"); rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rr.Code)
	}
	srv.zzaWaitTimeout = 20 * time.Millisecond
	for i := 0; i < cap(srv.zzaSem); i++ {
		srv.zzaSem <- struct{}{} // all slots busy
	}
	rr := get(srv, "/klein/zza.zip")
	if rr.Code != http.StatusUnprocessableEntity || !strings.Contains(rr.Body.String(), "240 × 240") {
		t.Fatalf("remembered result: got %d: %s", rr.Code, rr.Body.String())
	}
	for i := 0; i < cap(srv.zzaSem); i++ {
		<-srv.zzaSem
	}

	// Fixing the template drops the remembered result.
	p := filepath.Join(dir, "klein", "template.yaml")
	y, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	y = []byte(strings.Replace(strings.Replace(string(y), "width: 160", "width: 240", 1), "height: 80", "height: 240", 1))
	if err := os.WriteFile(p, y, 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(p, later, later); err != nil {
		t.Fatal(err)
	}
	if rr := get(srv, "/klein/zza.zip"); rr.Code != http.StatusOK {
		t.Fatalf("fixed template: got %d: %s", rr.Code, rr.Body.String())
	}
}

// "/zza.zip" is the source ZIP of a template called "zza", not an export.
func TestZZAExport_TemplateNamedZZAKeepsItsSourceZip(t *testing.T) {
	srv, dir := zzaTestServer(t)
	if err := os.Rename(filepath.Join(dir, "klein"), filepath.Join(dir, "zza")); err != nil {
		t.Fatal(err)
	}
	rr := get(srv, "/zza.zip")
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Disposition") != `attachment; filename="zza.zip"` {
		t.Fatalf("got %d, %q", rr.Code, rr.Header().Get("Content-Disposition"))
	}
	if rr := get(srv, "/zza/zza.zip"); rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("export of template zza: got %d", rr.Code)
	}
}

func TestZZAExport_Errors(t *testing.T) {
	srv, _ := zzaTestServer(t)

	rr := get(srv, "/klein/zza.zip")
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("incompatible: got %d: %s", rr.Code, rr.Body.String())
	}
	if body := rr.Body.String(); !strings.HasPrefix(body, "nicht ZZA-kompatibel:") || !strings.Contains(body, "240 × 240") {
		t.Errorf("incompatible body = %q", body)
	}
	if n := srv.zzaCache.Stats().FileCount; n != 0 {
		t.Errorf("incompatible template left %d cache files", n)
	}

	if rr := get(srv, "/gibt-es-nicht/zza.zip"); rr.Code != http.StatusNotFound {
		t.Errorf("unknown template: got %d", rr.Code)
	}
	if rr := get(srv, "/..%2Fx/zza.zip"); rr.Code != http.StatusBadRequest {
		t.Errorf("invalid name: got %d", rr.Code)
	}
	rrPost := httptest.NewRecorder()
	srv.ServeHTTP(rrPost, httptest.NewRequest(http.MethodPost, "/sbb-105-v1/zza.zip", nil))
	if rrPost.Code == http.StatusOK {
		t.Errorf("POST must not export")
	}
}

func TestZZAExport_SourceZipStillWorks(t *testing.T) {
	srv, _ := zzaTestServer(t)
	rr := get(srv, "/sbb-105-v1.zip")
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d", rr.Code)
	}
	if cd := rr.Header().Get("Content-Disposition"); cd != `attachment; filename="sbb-105-v1.zip"` {
		t.Errorf("Content-Disposition = %q", cd)
	}
}

func TestZZAExport_RedirectsHTTPToHTTPS(t *testing.T) {
	srv, _ := zzaTestServer(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sbb-105-v1/zza.zip", nil)
	req.Header.Set("X-Forwarded-Proto", "http")
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusMovedPermanently {
		t.Fatalf("got %d", rr.Code)
	}
}

func TestZZAExport_BusySlotsAnswer503(t *testing.T) {
	srv, _ := zzaTestServer(t)
	srv.zzaWaitTimeout = 20 * time.Millisecond
	for i := 0; i < cap(srv.zzaSem); i++ {
		srv.zzaSem <- struct{}{}
	}
	rr := get(srv, "/sbb-105-v1/zza.zip")
	if rr.Code != http.StatusServiceUnavailable || rr.Header().Get("Retry-After") == "" {
		t.Fatalf("got %d, Retry-After %q", rr.Code, rr.Header().Get("Retry-After"))
	}
}

// The image cache's cleanup and statistics must leave the zza cache alone:
// it lives in a subfolder of the same cache directory with its own limit.
func TestImageCacheIgnoresZZASubfolder(t *testing.T) {
	srv, _ := zzaTestServer(t)
	if rr := get(srv, "/sbb-105-v1/zza.zip"); rr.Code != http.StatusOK {
		t.Fatalf("got %d", rr.Code)
	}
	srv.cache.maxBytes = 1 // would evict everything it counts
	if err := srv.cache.cleanup(); err != nil {
		t.Fatal(err)
	}
	if n := srv.zzaCache.Stats().FileCount; n != 1 {
		t.Errorf("image cache cleanup removed the zza package (%d files left)", n)
	}
}

func TestZZACacheHonoursItsOwnLimit(t *testing.T) {
	srv, _ := zzaTestServer(t)
	if rr := get(srv, "/sbb-105-v1/zza.zip"); rr.Code != http.StatusOK {
		t.Fatalf("got %d", rr.Code)
	}
	srv.zzaCache.maxBytes = 1
	if err := srv.zzaCache.cleanup(); err != nil {
		t.Fatal(err)
	}
	if n := srv.zzaCache.Stats().FileCount; n != 0 {
		t.Errorf("%d files left above the limit", n)
	}
	// Still served, exported again.
	if rr := get(srv, "/sbb-105-v1/zza.zip"); rr.Code != http.StatusOK || rr.Header().Get("X-Cache") != "MISS" {
		t.Errorf("after eviction: %d, X-Cache %q", rr.Code, rr.Header().Get("X-Cache"))
	}
}

func TestDetailPage_ZZAButton(t *testing.T) {
	srv, _ := zzaTestServer(t) // server build: editor not enabled
	ok := get(srv, "/sbb-105-v1").Body.String()
	if !strings.Contains(ok, `id="zza-export-btn"`) || strings.Contains(ok, `id="zza-export-btn" class="tbtn" disabled`) {
		t.Errorf("compatible template: export button missing or disabled")
	}
	bad := get(srv, "/klein").Body.String()
	if !strings.Contains(bad, `id="zza-export-btn" class="tbtn" disabled`) {
		t.Errorf("incompatible template: button should be shown disabled")
	}
	if strings.Contains(bad, `"/zza.zip"`) {
		t.Errorf("incompatible template: page still wires the export request")
	}
	if !strings.Contains(bad, "240 × 240") {
		t.Errorf("incompatible template: reason not shown")
	}
}

// A cache file that is damaged or belongs to another template state is not
// served: the package is exported again and the file replaced.
func TestZZAExport_DamagedCacheFileIsNotServed(t *testing.T) {
	srv, _ := zzaTestServer(t)
	good := get(srv, "/sbb-105-v1/zza.zip")
	key := srv.zzaCache.Key("sbb-105-v1", nil)
	for _, junk := range [][]byte{
		nil,
		[]byte("short"),
		bytes.Repeat([]byte("0"), zzaStampLen), // stamp only, no package
		append(bytes.Repeat([]byte("0"), zzaStampLen), 'x'), // foreign stamp
	} {
		if err := srv.zzaCache.SetExt(key, zzaCacheExt, junk); err != nil {
			t.Fatal(err)
		}
		rr := get(srv, "/sbb-105-v1/zza.zip")
		if rr.Code != http.StatusOK || rr.Header().Get("X-Cache") != "MISS" || !bytes.Equal(rr.Body.Bytes(), good.Body.Bytes()) {
			t.Fatalf("junk %q: %d, X-Cache %q, same bytes %v", junk, rr.Code, rr.Header().Get("X-Cache"),
				bytes.Equal(rr.Body.Bytes(), good.Body.Bytes()))
		}
	}
}

func TestCacheSetExt_LeavesNoTemporaryFiles(t *testing.T) {
	srv, _ := zzaTestServer(t)
	for i := 0; i < 3; i++ {
		if err := srv.zzaCache.SetExt(srv.zzaCache.Key("a", nil), "bin", []byte("data")); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(srv.zzaCache.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || strings.HasPrefix(entries[0].Name(), ".tmp-") {
		t.Errorf("cache dir holds %d entries (%v)", len(entries), entries)
	}
	if got, ok := srv.zzaCache.GetExt(srv.zzaCache.Key("a", nil), "bin"); !ok || string(got) != "data" {
		t.Errorf("read back %q, %v", got, ok)
	}
}

// Many requests for the same template at once: all get the same package.
func TestZZAExport_ParallelRequests(t *testing.T) {
	srv, _ := zzaTestServer(t)
	const n = 12
	bodies := make([][]byte, n)
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rr := get(srv, "/sbb-105-v1/zza.zip")
			codes[i], bodies[i] = rr.Code, rr.Body.Bytes()
		}(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		if codes[i] != http.StatusOK || !bytes.Equal(bodies[i], bodies[0]) {
			t.Fatalf("request %d: code %d, same bytes %v", i, codes[i], bytes.Equal(bodies[i], bodies[0]))
		}
	}
	if c := srv.zzaCache.Stats().FileCount; c != 1 {
		t.Errorf("zza cache holds %d files", c)
	}
}

// When no slot frees up and nothing is known yet, the page offers the button
// instead of blocking or calling the template incompatible.
func TestDetailPage_ZZAButtonWhenBusy(t *testing.T) {
	srv, _ := zzaTestServer(t)
	srv.zzaWaitTimeout = 20 * time.Millisecond
	for i := 0; i < cap(srv.zzaSem); i++ {
		srv.zzaSem <- struct{}{}
	}
	for _, name := range []string{"/sbb-105-v1", "/klein"} {
		body := get(srv, name).Body.String()
		if !strings.Contains(body, `id="zza-export-btn" class="tbtn" title`) {
			t.Errorf("%s: button should be offered while the outcome is unknown", name)
		}
	}
}

// The detail page decides from the remembered result: no package read, no slot.
func TestDetailPage_ZZAButtonFromRememberedResult(t *testing.T) {
	srv, _ := zzaTestServer(t)
	get(srv, "/sbb-105-v1/zza.zip")
	get(srv, "/klein/zza.zip")
	if err := srv.zzaCache.Flush(); err != nil {
		t.Fatal(err)
	}
	srv.zzaWaitTimeout = 20 * time.Millisecond
	for i := 0; i < cap(srv.zzaSem); i++ {
		srv.zzaSem <- struct{}{}
	}
	if body := get(srv, "/sbb-105-v1").Body.String(); !strings.Contains(body, `id="zza-export-btn" class="tbtn" title`) {
		t.Error("compatible template: button missing")
	}
	if body := get(srv, "/klein").Body.String(); !strings.Contains(body, `id="zza-export-btn" class="tbtn" disabled`) {
		t.Error("incompatible template: button should be disabled from the remembered result")
	}
}

func TestPublicReasonsCarryNoServerPaths(t *testing.T) {
	srv, dir := zzaTestServer(t)
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := srv.publicReasons([]string{"Bild \"a.png\" nicht lesbar: open " + filepath.Join(abs, "klein", "a.png") + ": permission denied"})
	if strings.Contains(got[0], abs) || !strings.Contains(got[0], filepath.Join("klein", "a.png")) {
		t.Errorf("reason = %q", got[0])
	}
}

// A template that cannot be read is an error (500), not "incompatible", and
// is not remembered: it works again once the file is readable.
func TestZZAExport_UnreadableTemplateIsNotCalledIncompatible(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	srv, dir := zzaTestServer(t)
	p := filepath.Join(dir, "sbb-105-v1", "template.yaml")
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatal(err)
	}
	rr := get(srv, "/sbb-105-v1/zza.zip")
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("unreadable: got %d: %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), dir) {
		t.Errorf("error body leaks the path: %q", rr.Body.String())
	}
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if rr := get(srv, "/sbb-105-v1/zza.zip"); rr.Code != http.StatusOK {
		t.Fatalf("readable again: got %d: %s", rr.Code, rr.Body.String())
	}
}
