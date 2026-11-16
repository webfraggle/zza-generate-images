// Package devicerender is a cgo bridge to the C++ renderer core in
// device/zzarender (the same code that runs on the controller). Desktop only:
// the CGO-free server build must not import it.
package devicerender

/*
#cgo CFLAGS: -I${SRCDIR}/../../device/zzarender
#cgo CXXFLAGS: -std=c++17 -I${SRCDIR}/../../device/zzarender
#include <stdlib.h>
#include "zzarender_c.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"sort"
	"unsafe"

	"github.com/webfraggle/zza-generate-images/internal/devexport"
)

// Sentinel errors; the core's error text is appended via fmt.Errorf("%w: ...").
var (
	ErrNoMem    = errors.New("devicerender: out of memory budget")
	ErrFormat   = errors.New("devicerender: format error")
	ErrTemplate = errors.New("devicerender: template error")
	ErrData     = errors.New("devicerender: data error")
	ErrState    = errors.New("devicerender: invalid state")
)

// Renderer wraps a C renderer instance. Not safe for concurrent use.
type Renderer struct {
	c      *C.zza_renderer
	closed bool
}

// New creates a renderer with the given memory budget in bytes.
func New(memBudget int) *Renderer {
	if memBudget < 0 {
		memBudget = 0
	}
	return &Renderer{c: C.zza_new(C.size_t(memBudget))}
}

// Close releases the C renderer. Safe to call more than once.
func (r *Renderer) Close() {
	if r.c != nil {
		C.zza_delete(r.c)
		r.c = nil
	}
	r.closed = true
}

// usable reports why the renderer cannot be used: the core instance could not
// be created (allocation failure) or Close was called.
func (r *Renderer) usable() error {
	if r.c != nil {
		return nil
	}
	if r.closed {
		return fmt.Errorf("%w: renderer is closed", ErrState)
	}
	return fmt.Errorf("%w: creating the renderer failed", ErrNoMem)
}

// Load replaces the loaded package. The file data is copied into C memory for
// the call (cgo pointer rules), the core copies what it keeps into its budget.
// A failed load leaves the renderer without a package.
func (r *Renderer) Load(p *devexport.Package) error {
	if err := r.usable(); err != nil {
		return err
	}
	names := make([]string, 0, len(p.Files))
	for n := range p.Files {
		names = append(names, n)
	}
	sort.Strings(names)
	var (
		arr   *C.zza_file
		ptrs  []unsafe.Pointer
		files []C.zza_file
	)
	defer func() {
		for _, q := range ptrs {
			C.free(q)
		}
		if arr != nil {
			C.free(unsafe.Pointer(arr))
		}
	}()
	if len(names) > 0 {
		arr = (*C.zza_file)(C.malloc(C.size_t(len(names)) * C.size_t(unsafe.Sizeof(C.zza_file{}))))
		if arr == nil {
			return fmt.Errorf("%w: file table", ErrNoMem)
		}
		files = unsafe.Slice(arr, len(names))
	}
	for i, n := range names {
		cn := C.CString(n)
		ptrs = append(ptrs, unsafe.Pointer(cn))
		data := p.Files[n]
		var cd unsafe.Pointer
		if len(data) > 0 {
			cd = C.CBytes(data)
			ptrs = append(ptrs, cd)
		}
		files[i] = C.zza_file{name: cn, data: (*C.uchar)(cd), len: C.size_t(len(data))}
	}
	return r.errFor(C.zza_load(r.c, arr, C.int(len(names))))
}

// Version returns the core version string, e.g. "zzarender/1".
func Version() string { return C.GoString(C.zza_version()) }

// MemUsed returns the bytes currently allocated through the budget.
func (r *Renderer) MemUsed() int { return int(C.zza_mem_used(r.c)) }

// MemPeak returns the peak budget usage in bytes.
func (r *Renderer) MemPeak() int { return int(C.zza_mem_peak(r.c)) }

// errFor maps a ZZA_* status code to a Go error (nil for ZZA_OK).
func (r *Renderer) errFor(code C.int) error {
	var base error
	switch code {
	case C.ZZA_OK:
		return nil
	case C.ZZA_ERR_FORMAT:
		base = ErrFormat
	case C.ZZA_ERR_TEMPLATE:
		base = ErrTemplate
	case C.ZZA_ERR_NOMEM:
		base = ErrNoMem
	case C.ZZA_ERR_DATA:
		base = ErrData
	case C.ZZA_ERR_STATE:
		base = ErrState
	default:
		return fmt.Errorf("devicerender: unknown status %d", int(code))
	}
	if msg := C.GoString(C.zza_error(r.c)); msg != "" {
		return fmt.Errorf("%w: %s", base, msg)
	}
	return base
}
