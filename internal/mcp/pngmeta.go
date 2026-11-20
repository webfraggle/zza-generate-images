package mcp

import (
	"bytes"
	"encoding/binary"
)

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// pngMetaChunks are ancillary PNG chunks that can carry personal data
// (comments, author, GPS via EXIF).
var pngMetaChunks = map[string]bool{"tEXt": true, "iTXt": true, "zTXt": true, "eXIf": true}

// pngHasMetadata reports whether data is a PNG with text or EXIF chunks.
// Such files are re-encoded before being stored (templates are shared as
// ZIP); other PNGs are stored byte for byte.
func pngHasMetadata(data []byte) bool {
	if !bytes.HasPrefix(data, pngSignature) {
		return false
	}
	p := len(pngSignature)
	for p+8 <= len(data) {
		n := int(binary.BigEndian.Uint32(data[p : p+4]))
		typ := string(data[p+4 : p+8])
		if pngMetaChunks[typ] {
			return true
		}
		if typ == "IEND" || n < 0 || n > len(data)-p-12 {
			return false
		}
		p += 12 + n // length + type + data + CRC
	}
	return false
}
