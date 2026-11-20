package mcp

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withPNGChunk inserts a chunk right after IHDR (8 byte signature + 25 byte IHDR).
func withPNGChunk(t *testing.T, png []byte, typ string, data []byte) []byte {
	t.Helper()
	var c bytes.Buffer
	_ = binary.Write(&c, binary.BigEndian, uint32(len(data)))
	c.WriteString(typ)
	c.Write(data)
	_ = binary.Write(&c, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(typ), data...)))
	out := append([]byte{}, png[:33]...)
	out = append(out, c.Bytes()...)
	return append(out, png[33:]...)
}

func TestPNGHasMetadata(t *testing.T) {
	plain := pngBytes(t, solid(4, 4, color.NRGBA{1, 2, 3, 255}))
	if pngHasMetadata(plain) {
		t.Error("plain PNG reported as having metadata")
	}
	for _, typ := range []string{"tEXt", "iTXt", "zTXt", "eXIf"} {
		if !pngHasMetadata(withPNGChunk(t, plain, typ, []byte("k\x00GPS-SECRET"))) {
			t.Errorf("%s not detected", typ)
		}
	}
	if pngHasMetadata([]byte("not a png")) {
		t.Error("garbage reported as metadata")
	}
}

// PNG metadata chunks (text, EXIF) are removed by re-encoding; templates are
// shared as ZIP. PNGs without such chunks stay byte-identical.
func TestPNGMetadataStripped(t *testing.T) {
	dir := copyTemplates(t, "sbb-105-v1")
	cs := newSession(t, dir)
	pb := withPNGChunk(t, pngBytes(t, solid(16, 16, color.NRGBA{9, 99, 9, 255})), "tEXt", []byte("Comment\x00GPS-SECRET"))

	requireOK(t, callTool(t, cs, "upload_image", map[string]any{"template": "sbb-105-v1", "name": "u.png", "data_base64": base64.StdEncoding.EncodeToString(pb)}))
	if got := readDisk(t, dir, "sbb-105-v1", "u.png"); strings.Contains(got, "GPS-SECRET") {
		t.Error("upload_image kept PNG text chunk")
	}

	src := filepath.Join(t.TempDir(), "p.png")
	if err := os.WriteFile(src, pb, 0o644); err != nil {
		t.Fatal(err)
	}
	requireOK(t, callTool(t, cs, "import_image", map[string]any{"template": "sbb-105-v1", "path": src}))
	if got := readDisk(t, dir, "sbb-105-v1", "p.png"); strings.Contains(got, "GPS-SECRET") {
		t.Error("import_image kept PNG text chunk")
	}
}
