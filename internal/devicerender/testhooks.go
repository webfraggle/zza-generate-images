package devicerender

/*
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
