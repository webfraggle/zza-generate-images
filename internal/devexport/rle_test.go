package devexport

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestRLERoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	cases := map[string][]byte{}
	for _, bpp := range []int{1, 4} {
		for _, n := range []int{1, 2, 3, 127, 128, 129, 130, 300, 1000} {
			random := make([]byte, n*bpp)
			rng.Read(random)
			flat := bytes.Repeat([]byte{7, 8, 9, 10}[:bpp], n)
			grad := make([]byte, n*bpp)
			for i := range grad {
				grad[i] = byte(i / bpp * 255 / n)
			}
			// runs with a few random break-ins
			mixed := bytes.Clone(flat)
			for i := 0; i < n/7; i++ {
				mixed[rng.Intn(len(mixed))] ^= byte(rng.Intn(255) + 1)
			}
			for name, d := range map[string][]byte{"random": random, "flat": flat, "grad": grad, "mixed": mixed} {
				cases[name+string(rune('0'+bpp))+string(rune(n))] = append([]byte{byte(bpp)}, d...)
			}
		}
	}
	for name, c := range cases {
		bpp, d := int(c[0]), c[1:]
		enc := EncodeRLE(d, bpp)
		dec, err := DecodeRLE(enc, bpp, len(d)/bpp)
		if err != nil || !bytes.Equal(dec, d) {
			t.Fatalf("%q: round trip failed (%v)", name, err)
		}
		if len(enc) > len(d)+len(d)/bpp/128+1 {
			t.Errorf("%q: expansion %d -> %d too large", name, len(d), len(enc))
		}
	}
	if enc := EncodeRLE(bytes.Repeat([]byte{1, 2, 3, 4}, 129), 4); len(enc) != 5 || enc[0] != 255 {
		t.Errorf("max run encoding = %v", enc)
	}
}

func TestEncodeImageRawWhenNotSmaller(t *testing.T) {
	b := EncodeImage(gradient(16, 16))
	img, err := DecodeImage(b)
	if err != nil || img.Bounds().Dx() != 16 {
		t.Fatalf("%v", err)
	}
	if b[4] != 0 {
		t.Errorf("gradient with alpha should stay raw, enc=%d", b[4])
	}
	flat := EncodeImage(imgFlat(64, 64))
	if flat[4] != 1 || len(flat) > 200 {
		t.Errorf("flat image enc=%d len=%d", flat[4], len(flat))
	}
	if _, err := DecodeImage(flat); err != nil {
		t.Fatal(err)
	}
}
