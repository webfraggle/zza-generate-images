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
)

// errZZABusy means no export slot became free in time.
var errZZABusy = errors.New("all export slots busy")

// zzaResult is the remembered outcome for a template that cannot be exported.
type zzaResult struct {
	stamp   string
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
		if !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
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
		return z, true, nil
	}
	if reasons, ok := s.zzaKnownIncompatible(name, stamp); ok {
		return nil, false, &devexport.Incompatible{Reasons: reasons}
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
			s.zzaRememberIncompatible(name, stamp, inc.Reasons)
		}
		return nil, false, err
	}
	z, err := pkg.ZIP()
	if err != nil {
		return nil, false, err
	}
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

// zzaKnownIncompatible reports the remembered reasons if this exact template
// state was already found incompatible, so repeated requests cost no export.
func (s *Server) zzaKnownIncompatible(name, stamp string) ([]string, bool) {
	s.zzaMu.Lock()
	defer s.zzaMu.Unlock()
	r, ok := s.zzaIncompat[name]
	if !ok || r.stamp != stamp {
		return nil, false
	}
	return r.reasons, true
}

// zzaRememberIncompatible keeps one entry per template. Only names of existing
// template folders get here, so the map is bounded by the templates on disk
// (the cap guards against folders that come and go).
func (s *Server) zzaRememberIncompatible(name, stamp string, reasons []string) {
	s.zzaMu.Lock()
	defer s.zzaMu.Unlock()
	if len(s.zzaIncompat) >= 1024 {
		s.zzaIncompat = map[string]zzaResult{}
	}
	s.zzaIncompat[name] = zzaResult{stamp: stamp, reasons: reasons}
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
// known is false when that could not be determined right now (slots busy);
// the page then offers the button and a click reports the outcome.
func (s *Server) zzaCompat(name string) (ok bool, reasons []string, known bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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
