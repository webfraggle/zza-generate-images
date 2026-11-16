package devicerender

import (
	"crypto/sha256"
	"math/rand"
	"testing"
)

const (
	zzaOK    = 0
	zzaNoMem = 3
)

func TestVersion(t *testing.T) {
	if got := Version(); got != "zzarender/1" {
		t.Fatalf("Version() = %q", got)
	}
}

func TestBudgetNoMem(t *testing.T) {
	r := New(1024)
	defer r.Close()
	if got := testAlloc(r, 4096); got != zzaNoMem {
		t.Fatalf("testAlloc(4096) = %d, want ZZA_ERR_NOMEM", got)
	}
	if r.MemUsed() != 0 {
		t.Fatalf("MemUsed after failed alloc = %d", r.MemUsed())
	}
	if got := testAlloc(r, 512); got != zzaOK {
		t.Fatalf("testAlloc(512) = %d, want ZZA_OK", got)
	}
	if r.MemPeak() < 512 {
		t.Fatalf("MemPeak = %d, want >= 512", r.MemPeak())
	}
	if r.MemUsed() != 0 {
		t.Fatalf("MemUsed after free = %d", r.MemUsed())
	}
}

func TestSha256(t *testing.T) {
	big := make([]byte, 1<<20)
	rand.New(rand.NewSource(1)).Read(big)
	cases := map[string][]byte{"empty": nil, "abc": []byte("abc"), "55": make([]byte, 55), "56": make([]byte, 56), "64": make([]byte, 64), "1MB": big}
	for name, in := range cases {
		if got, want := testSHA256(in), sha256.Sum256(in); got != want {
			t.Errorf("%s: got %x want %x", name, got, want)
		}
	}
}
