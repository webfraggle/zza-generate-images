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
// 2000-01-01 00:00:00, so equal input gives equal bytes. Local file headers are
// complete (no data descriptor, general-purpose flag bit 3 clear: CRC-32 and both
// sizes are in the local header), so a sequential reader works without the
// central directory. ReadZIP also accepts
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
//     pattern that is not a quoted literal is rejected. The key is the pattern
//     text exactly as the unquoted filter argument (the characters between the
//     quotes, no escape processing), the same string the device finds when it
//     evaluates match('…', n).
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
// above over 1-byte values); off is then a BYTE offset into the compressed mask
// area (start of that glyph's RLE stream) and the stream decodes to exactly w*h
// bytes (the decoder stops after w*h bytes; the next glyph's stream follows
// directly). Bit 0 clear: off is a byte offset of w*h raw bytes.
//
// # Text layout on the device (contract for the C++ loader)
//
// Units: adv and kern are natural (unscaled) 26.6 values (64 = 1 px). ascent,
// descent, height are whole pixels. bx/by/w/h are final pixels (scale_x/scale_y
// already applied to the mask) and must not be scaled again.
//
//   - ascent = round(natural ascent * scale_y); height = round(natural line
//     height * scale_y); descent is the natural, UNscaled descent (scale_y is
//     not applied). line_height is not in the file: at run time
//     lineHeight = round(height * line_height) when line_height > 0 && != 1,
//     else height. With scale_y as well, Go multiplies the natural height
//     first: lineHeight = round(round(H * line_height) * scale_y), H recovered
//     from height (the value with round(H * scale_y) == height closest to
//     height / scale_y; exact for scale_y >= 1). First-line top = y; baseline = y + ascent (valign: baseline:
//     baseline = y).
//   - Pen: kept in natural 26.6 as an integer sum over the drawn glyphs
//     pen = Σ (adv(g) + kern(prev, g)); kern(prev, g) is added before g when a
//     previous glyph exists, and is 0 if the pair is not in the table.
//     Total line width in natural pixels is measured from the same sum.
//   - Glyph placement without scale_x/scale_y: gx = x + round(pen_before_g / 64),
//     pen_before_g including the kern of g. The mask's top-left is at
//     (gx + bx, baselineY + by). scale_x/scale_y of 0 mean 1.
//   - With scale_x or scale_y != 1 the core follows Go's drawTextLine, which
//     draws the line offscreen with a margin pad = A (natural ascent; with
//     scale_y recovered from the stored ascent) and resamples it to
//     dw = ceil((W + 2A) * sx), dh = ceil((3A + descent) * sy) at
//     dx = x - round(A * sx), dy = baselineY - round(2A * sy):
//     gx = round(dx + (A + pen_before_g / 64) * dw / (W + 2A)) and the
//     baseline is round(dy + 2A * dh / (3A + descent)); round = floor(v + 0.5).
//     Lines far wider than the canvas are first cut like visibleRun.
//     Natural line width W = round(pen_total / 64) (like font.MeasureString(...).Round());
//     the on-canvas width used for alignment and <id>.width is ceil(W * scale_x)
//     (W when scale_x == 1).
//   - Subpixel: masks are rasterised at subpixel 0, so glyphs sit on whole
//     pixels; this is the accepted deviation from Go (checked by the "near"
//     comparison, not bit-exact).
//   - Missing glyph: a code point that is > 0xFF, < 0x20 or absent from the
//     table is replaced by '?' (always present) before layout, so '?' provides
//     advance, mask and kerning for it.
//
// Glyph set per font combination (file, size, scale_x, scale_y): all characters
// of its constant texts; as soon as one text of the combination contains "{{",
// the full range 0x20–0xFF. '?' is always included (stand-in for a missing glyph).
package devexport
