// Package devexport converts a template into a package for the ZZA controller
// and decides whether the template is compatible (Incompatible). It is pure Go
// and CGO-free. The device core (device/zzarender) reads exactly these formats.
//
// All multi-byte integers are little-endian. File names are flat and at most
// 12 characters.
//
// # Package (ZIP)
//
// Method 0 (stored), manifest.json first, the rest sorted by name, timestamp
// 2000-01-01 00:00:00, so equal input gives equal bytes. ReadZIP also accepts
// a ZIP with a single top-level folder.
//
//	manifest.json  {"format":1,"name":…,"canvas":[240,240],
//	               "files":[{"n":name,"s":size,"h":sha256hex},…],   (all files except manifest.json)
//	               "features":[…]}                                  ("splitflap","frames","regex","images")
//	t.json         the template as JSON, see below
//	iN.img         image N (N = 0, 1, …)
//	fN.fnt         glyph set N
//
// # t.json
//
// template.yaml converted to JSON (same keys and structure) with these rules:
//
//   - Fields the renderer reads as text (type, if, elif, file, color, value, font,
//     align, valign, split_by, var, split, id, rotate, reel entries, meta strings)
//     are JSON strings with the raw YAML text, never numbers or booleans.
//   - x, y, width, height, max_width: a JSON integer, or the string "{{expr}}".
//   - A conditional map under value, color or rotate is
//     {"_cond":[["if",expr],["then",v],["elif",expr],["then",v],…,["else",v]]},
//     the pairs in YAML order (keys may repeat in YAML).
//   - A bare `else:` on a layer is true.
//   - Text layers carry "_f":"fN" (the glyph set of their font, size and scales).
//   - Image layers with constant width/height (or none) carry "_i":"iN", the
//     image already scaled like the Go renderer does (width/height absent or 0
//     keep the original). Image layers with a dynamic width/height carry
//     "_io":"iN" instead, the original image to scale at run time.
//   - Top-level "_re":{"<pattern>":[start,numCap,[[op,out,arg,[runes…]],…]]}
//     holds the compiled program (Prog, see CompileRegex) of every constant
//     pattern of a match filter. The device cannot compile regexes, so a
//     pattern that is not a quoted literal is rejected.
//
// # iN.img
//
//	u16 w, u16 h, u8 enc, data
//	enc 0: w*h*4 bytes RGBA8888, non-premultiplied, row-major
//	enc 1: RLE over 4-byte pixels, decodes to exactly w*h pixels
//
// RLE: control byte c < 128: c+1 literal pixels follow; c >= 128: the next pixel
// repeats c-126 times (2..129). The encoder uses enc 1 only if smaller than raw.
//
// # fN.fnt
//
// Rasterised at subpixel 0 with opentype.NewFace(size, DPI 72). With scale_x or
// scale_y != 1 each mask is scaled (CatmullRom, w = ceil(w*sx), h = ceil(h*sy))
// and bx/by are scaled and rounded; ascent and height are scaled by scale_y.
//
//	header (16 bytes): "ZF1", u8 flags, i16 ascent, i16 descent, i16 height,
//	                   u16 nGlyphs, u32 nKern
//	nGlyphs entries (17 bytes each), sorted by code point:
//	                   u8 cp, i32 adv (26.6, natural, unscaled), i16 bx, i16 by
//	                   (mask top-left relative to pen at baseline, scaled),
//	                   u16 w, u16 h, u32 off (into the mask area; w*h = 0: no mask)
//	nKern entries (6 bytes each), sorted by key: u16 key = left<<8|right,
//	                   i32 kern (26.6, natural)
//	mask area: A8 masks
//
// flags bit 0: every mask is stored RLE-compressed on its own (same RLE as
// above over 1-byte values, decoding to exactly w*h bytes); off then points
// into the compressed stream. Bit 0 clear: raw w*h bytes per mask.
//
// Glyph set per font combination (file, size, scale_x, scale_y): all characters
// of its constant texts; as soon as one text of the combination contains "{{",
// the full range 0x20–0xFF. '?' is always included (stand-in for a missing glyph).
package devexport
