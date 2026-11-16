package devexport

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// ZIP returns the package as a reproducible ZIP: method 0 (stored), manifest.json
// first, then the remaining files sorted by name, fixed timestamp 2000-01-01 00:00:00.
func (p *Package) ZIP() ([]byte, error) {
	names := make([]string, 0, len(p.Files))
	for n := range p.Files {
		if n != "manifest.json" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	if _, ok := p.Files["manifest.json"]; ok {
		names = append([]string{"manifest.json"}, names...)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range names {
		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:         n,
			Method:       zip.Store,
			ModifiedDate: (2000-1980)<<9 | 1<<5 | 1, // 2000-01-01
			ModifiedTime: 0,
		})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(p.Files[n]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

const maxUnpackedBytes = 32 << 20

// ReadZIP reads a package ZIP. Directories are ignored and files are flattened
// to their base name, so a ZIP with a top-level folder (as zipped by file
// managers or written by the firmware backup) reads like a flat one.
func ReadZIP(b []byte) (*Package, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}
	p := &Package{Files: map[string][]byte{}}
	total := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || strings.HasPrefix(f.Name, "__MACOSX/") {
			continue
		}
		base := path.Base(f.Name)
		if strings.HasPrefix(base, ".") {
			continue
		}
		if _, dup := p.Files[base]; dup {
			return nil, fmt.Errorf("zip: duplicate file %q", base)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxUnpackedBytes+1))
		rc.Close()
		if err != nil {
			return nil, err
		}
		if total += len(data); total > maxUnpackedBytes {
			return nil, fmt.Errorf("zip: unpacked size exceeds %d MB", maxUnpackedBytes>>20)
		}
		p.Files[base] = data
	}
	m, ok := p.Files["manifest.json"]
	if !ok {
		return nil, fmt.Errorf("zip: manifest.json missing")
	}
	var mf struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(m, &mf); err != nil {
		return nil, fmt.Errorf("zip: manifest.json: %w", err)
	}
	p.Name = mf.Name
	return p, nil
}
