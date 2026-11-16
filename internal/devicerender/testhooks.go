package devicerender

/*
#include <stdlib.h>
#include "zzarender_c.h"
*/
import "C"

import "unsafe"

// Unexported wrappers around the zza_test_* C functions. import "C" is not
// allowed in _test.go files, so tests reach the core through these.

// testAlloc allocates and frees n bytes through the budget; returns the ZZA_* code.
func testAlloc(r *Renderer, n int) int {
	return int(C.zza_test_alloc(r.c, C.size_t(n)))
}

// testSHA256 hashes data with the core's SHA-256.
func testSHA256(data []byte) [32]byte {
	var out [32]byte
	var p *C.uchar
	if len(data) > 0 {
		p = (*C.uchar)(unsafe.Pointer(&data[0]))
	}
	C.zza_test_sha256(p, C.size_t(len(data)), (*C.uchar)(unsafe.Pointer(&out[0])))
	return out
}

// zzaTime mirrors zza_time for the test hooks.
type zzaTime struct{ Year, Month, Day, Hour, Minute, Second, Weekday int }

// testEval evaluates expr (as condition if isCond) with the C++ evaluator.
// regexJSON is a `_re` table {"<pattern>": <program>, …} for match ("" = none).
// The second result is a ZZA_ERR_* code (0 = ok).
func testEval(dataJSON, regexJSON string, now zzaTime, frame, frames, delayCS int, expr string, isCond bool) (string, int) {
	cData := C.CString(dataJSON)
	defer C.free(unsafe.Pointer(cData))
	var cRegex *C.char
	if regexJSON != "" {
		cRegex = C.CString(regexJSON)
		defer C.free(unsafe.Pointer(cRegex))
	}
	cExpr := C.CString(expr)
	defer C.free(unsafe.Pointer(cExpr))
	t := C.zza_time{
		year: C.int(now.Year), month: C.int(now.Month), day: C.int(now.Day),
		hour: C.int(now.Hour), minute: C.int(now.Minute), second: C.int(now.Second),
		weekday: C.int(now.Weekday),
	}
	cond := C.int(0)
	if isCond {
		cond = 1
	}
	buf := make([]byte, 4096)
	for {
		n := int(C.zza_test_eval(cData, cRegex, &t, C.int(frame), C.int(frames), C.int(delayCS), cExpr, cond,
			(*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf))))
		if n < 0 {
			return "", -n
		}
		if n < len(buf) {
			return string(buf[:n]), 0
		}
		buf = make([]byte, n+1)
	}
}

// testRegex runs a program (devexport.Prog JSON) on in and returns capture
// group g with Go matchGroup semantics; the second result is a ZZA_* code.
func testRegex(progJSON, in string, group int) (string, int) {
	cProg := C.CString(progJSON)
	defer C.free(unsafe.Pointer(cProg))
	b := []byte(in)
	var p *C.char
	if len(b) > 0 {
		p = (*C.char)(unsafe.Pointer(&b[0]))
	}
	out := make([]byte, len(b)+1) // a group is a substring of in
	n := int(C.zza_test_regex(cProg, p, C.size_t(len(b)), C.int(group),
		(*C.char)(unsafe.Pointer(&out[0])), C.size_t(len(out))))
	if n < 0 {
		return "", -n
	}
	return string(out[:n]), 0
}

// testIntExpr evaluates a coordinate expression; the second result is a ZZA_* code.
func testIntExpr(expr, varsJSON string) (int, int) {
	cExpr := C.CString(expr)
	defer C.free(unsafe.Pointer(cExpr))
	cVars := C.CString(varsJSON)
	defer C.free(unsafe.Pointer(cVars))
	var out C.int
	rc := int(C.zza_test_intexpr(cExpr, cVars, &out))
	return int(out), rc
}

// testLatin1 converts UTF-8 to Latin-1 bytes; ok is false in strict mode when a
// rune above U+00FF is present.
func testLatin1(s string, strict bool) ([]byte, bool) {
	b := []byte(s)
	out := make([]byte, len(b)+1)
	var p *C.char
	if len(b) > 0 {
		p = (*C.char)(unsafe.Pointer(&b[0]))
	}
	st := C.int(0)
	if strict {
		st = 1
	}
	n := int(C.zza_test_latin1(p, C.size_t(len(b)), st, (*C.uchar)(unsafe.Pointer(&out[0])), C.size_t(len(out))))
	if n < 0 {
		return nil, false
	}
	return out[:n], true
}

// testHash returns the core's FNV-1a of the decoded data of image/font id ("i0", "f1").
func testHash(r *Renderer, id string) (uint32, int) {
	cid := C.CString(id)
	defer C.free(unsafe.Pointer(cid))
	var out C.uint
	rc := int(C.zza_test_hash(r.c, cid, &out))
	return uint32(out), rc
}
