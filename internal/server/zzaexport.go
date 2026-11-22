package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
	"github.com/webfraggle/zza-generate-images/internal/version"
)

// GET /{template}/zza.zip serves the template as a package for the ZZA
// controller (the same bytes as `zza export` and the desktop's "Für ZZA
// exportieren"). The route is public, so the work is bounded: packages are
// cached on disk (their own folder and size limit next to the image cache) and
// at most maxConcurrentZZAExports exports run at once.
const (
	zzaExportSuffix = "/zza.zip"

	// zzaCacheSubdir is the folder of the package cache inside CACHE_DIR.
	zzaCacheSubdir = "zza"
	// defaultZZACacheMaxSizeMB applies when the config leaves the limit unset.
	defaultZZACacheMaxSizeMB = 50

	maxConcurrentZZAExports = 2
	defaultZZAWaitTimeout   = 10 * time.Second
	// A cache file is the fingerprint hash (zzaStampLen hex characters) followed
	// by the ZIP, so it is not a plain .zip.
	zzaCacheExt = "zzapkg"
	zzaStampLen = 64

	// zzaCompatWait bounds how long a detail page waits to learn whether its
	// template is exportable when that is not known yet.
	zzaCompatWait = 2 * time.Second
	// maxZZAKnown caps the remembered results (one per template).
	maxZZAKnown = 1024
)

// errZZABusy means no export slot became free in time.
var errZZABusy = errors.New("all export slots busy")

// zzaResult is the remembered outcome of the last export of a template:
// exportable (ok), or not, with the reasons. It holds for the template state
// that stamp stands for.
type zzaResult struct {
	stamp   string
	ok      bool
	reasons []string
}

// zzaFingerprint describes the template's current files (name, size, mtime of
// every regular file) plus the exporter that reads them. A changed template
// or a new server version gives a new fingerprint and so a fresh export.
func zzaFingerprint(templatesDir, name string) ([]byte, error) {
	dir := filepath.Join(templatesDir, name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// os.Stat follows symlinks like the exporter does when it reads the
		// file, so a changed link target changes the fingerprint too.
		info, err := os.Stat(filepath.Join(dir, e.Name()))
		if err != nil || info.IsDir() {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s\x00%d\x00%d", e.Name(), info.Size(), info.ModTime().UnixNano()))
	}
	sort.Strings(lines)
	head := fmt.Sprintf("zza\x00%d\x00%s", devexport.FormatVersion, version.Version)
	return []byte(head + "\n" + strings.Join(lines, "\n")), nil
}

// zzaPackage returns the ZZA package of a template and whether it came from
// the cache. Errors: fs.ErrNotExist (no such template), *devexport.Incompatible,
// errZZABusy, ctx.Err(), or an internal error.
func (s *Server) zzaPackage(ctx context.Context, name string) (data []byte, hit bool, err error) {
	fp, err := zzaFingerprint(s.templatesDir, name)
	if err != nil {
		return nil, false, err
	}
	// One cache file per template (the key depends on the name only), stamped
	// with the fingerprint it was built from. A changed template overwrites its
	// file, so the cache never holds more than one package per template.
	key := s.zzaCache.Key(name, nil)
	stamp := s.zzaCache.Key(name, fp) // 64 hex characters
	if z, ok := s.zzaCached(key, stamp); ok {
		s.zzaRemember(name, zzaResult{stamp: stamp, ok: true})
		return z, true, nil
	}
	if r, known := s.zzaKnownResult(name, stamp); known && !r.ok {
		return nil, false, &devexport.Incompatible{Reasons: r.reasons}
	}

	wait := time.NewTimer(s.zzaWaitTimeout)
	defer wait.Stop()
	select {
	case s.zzaSem <- struct{}{}:
		defer func() { <-s.zzaSem }()
	case <-ctx.Done():
		return nil, false, ctx.Err()
	case <-wait.C:
		return nil, false, errZZABusy
	}
	// Another request may have exported the same template while this one waited.
	if z, ok := s.zzaCached(key, stamp); ok {
		return z, true, nil
	}

	pkg, err := devexport.Export(s.templatesDir, name)
	if err != nil {
		var inc *devexport.Incompatible
		if errors.As(err, &inc) {
			// The reasons go to the public: no server paths in them.
			inc = &devexport.Incompatible{Reasons: s.publicReasons(inc.Reasons)}
			s.zzaRemember(name, zzaResult{stamp: stamp, reasons: inc.Reasons})
			return nil, false, inc
		}
		return nil, false, err
	}
	z, err := pkg.ZIP()
	if err != nil {
		return nil, false, err
	}
	s.zzaRemember(name, zzaResult{stamp: stamp, ok: true})
	if err := s.zzaCache.SetExt(key, zzaCacheExt, append([]byte(stamp), z...)); err != nil {
		log.Printf("zza-export: cache %q: %v", name, err) // still serve it
	}
	return z, false, nil
}

// zzaCached returns the cached package under key if it was built from the
// template state that stamp stands for.
func (s *Server) zzaCached(key, stamp string) ([]byte, bool) {
	b, ok := s.zzaCache.GetExt(key, zzaCacheExt)
	if !ok || len(b) <= zzaStampLen || string(b[:zzaStampLen]) != stamp {
		return nil, false
	}
	return b[zzaStampLen:], true
}

// zzaKnownResult returns the remembered outcome if it is for this exact
// template state, so repeated requests for an incompatible template cost no
// export and the detail page needs no package to decide about its button.
func (s *Server) zzaKnownResult(name, stamp string) (zzaResult, bool) {
	s.zzaMu.Lock()
	defer s.zzaMu.Unlock()
	r, ok := s.zzaKnown[name]
	if !ok || r.stamp != stamp {
		return zzaResult{}, false
	}
	return r, true
}

// zzaRemember keeps one entry per template. Only names of existing template
// folders get here, so the map is bounded by the templates on disk (the cap
// guards against folders that come and go).
func (s *Server) zzaRemember(name string, r zzaResult) {
	s.zzaMu.Lock()
	defer s.zzaMu.Unlock()
	if _, exists := s.zzaKnown[name]; !exists && len(s.zzaKnown) >= maxZZAKnown {
		s.zzaKnown = map[string]zzaResult{}
	}
	s.zzaKnown[name] = r
}

// publicReasons removes the templates directory from the exporter's messages
// (error texts of the standard library carry full paths).
func (s *Server) publicReasons(reasons []string) []string {
	out := make([]string, len(reasons))
	abs, absErr := filepath.Abs(s.templatesDir)
	for i, r := range reasons {
		if absErr == nil {
			r = strings.ReplaceAll(r, abs+string(filepath.Separator), "")
		}
		out[i] = strings.ReplaceAll(r, filepath.Clean(s.templatesDir)+string(filepath.Separator), "")
	}
	return out
}

// handleZZAExport answers GET /{template}/zza.zip. The caller has extracted
// the template name from the URL.
func (s *Server) handleZZAExport(w http.ResponseWriter, r *http.Request, name string) {
	if err := renderer.ValidateTemplateName(name); err != nil {
		http.Error(w, "invalid template name", http.StatusBadRequest)
		return
	}
	z, hit, err := s.zzaPackage(r.Context(), name)
	var inc *devexport.Incompatible
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		http.Error(w, "template not found", http.StatusNotFound)
		return
	case errors.As(err, &inc):
		http.Error(w, "nicht ZZA-kompatibel:\n"+strings.Join(inc.Reasons, "\n"), http.StatusUnprocessableEntity)
		return
	case errors.Is(err, errZZABusy):
		w.Header().Set("Retry-After", "5")
		http.Error(w, "all export slots busy, try again", http.StatusServiceUnavailable)
		return
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		http.Error(w, "request cancelled while waiting for an export slot", http.StatusServiceUnavailable)
		return
	default:
		log.Printf("zza-export: %q: %v", name, err)
		http.Error(w, "ZZA export failed", http.StatusInternalServerError)
		return
	}

	cacheState := "MISS"
	if hit {
		cacheState = "HIT"
	}
	// name passed ValidateTemplateName, so it is safe inside the quotes.
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`-zza.zip"`)
	// The package changes whenever the template does; browsers must not keep it.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Length", strconv.Itoa(len(z)))
	w.Header().Set("X-Cache", cacheState)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(z)
}

// zzaCompat reports for the detail page whether the template can be exported.
// It answers from the remembered result when there is one; otherwise it
// exports once (at most zzaCompatWait). known is false when that could not be
// determined right now (slots busy, read error); the page then offers the
// button and a click reports the outcome.
func (s *Server) zzaCompat(ctx context.Context, name string) (ok bool, reasons []string, known bool) {
	if fp, err := zzaFingerprint(s.templatesDir, name); err == nil {
		if r, found := s.zzaKnownResult(name, s.zzaCache.Key(name, fp)); found {
			return r.ok, r.reasons, true
		}
	}
	ctx, cancel := context.WithTimeout(ctx, zzaCompatWait)
	defer cancel()
	_, _, err := s.zzaPackage(ctx, name)
	var inc *devexport.Incompatible
	switch {
	case err == nil:
		return true, nil, true
	case errors.As(err, &inc):
		return false, inc.Reasons, true
	default:
		return false, nil, false
	}
}
