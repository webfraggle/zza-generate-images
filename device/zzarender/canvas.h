// RGB888 canvas and pixel operations: blend, image blit, Catmull-Rom scaling,
// bilinear rotation, copy and rectangle fill. Ports of the drawing calls of
// internal/renderer (image/draw, x/image/draw) with float and fixed-point math.
#pragma once
#include <cstddef>
#include <cstdint>

#include "mem.h"
#include "package.h"

namespace zza {

// Opaque RGB888 canvas, row-major, stride w*3. The pixels are not owned.
struct Canvas {
  int w, h;
  uint8_t* px;
  uint8_t* at(int x, int y) const { return px + 3 * ((size_t)y * w + x); }
};

struct Rgba {
  uint8_t r, g, b, a;
};

// round(v / 255) for v <= 255*255*2, without division.
inline uint32_t div255(uint32_t v) {
  v += 128;
  return (v + (v >> 8)) >> 8;
}

// Blends (r,g,b) with alpha a (0..255) over the opaque pixel d.
inline void blendPx(uint8_t* d, uint32_t r, uint32_t g, uint32_t b, uint32_t a) {
  if (a == 0) return;
  if (a >= 255) {
    d[0] = (uint8_t)r;
    d[1] = (uint8_t)g;
    d[2] = (uint8_t)b;
    return;
  }
  uint32_t ia = 255 - a;
  d[0] = (uint8_t)div255(r * a + d[0] * ia);
  d[1] = (uint8_t)div255(g * a + d[1] * ia);
  d[2] = (uint8_t)div255(b * a + d[2] * ia);
}

// A rectangle of an RGBA8888 (non-premultiplied) image of full size w×h:
// pixel (x, y) of the full image, ox <= x < ox+rw, oy <= y < oy+rh, is at
// px[4*((y-oy)*stride + (x-ox))].
struct ImgView {
  const uint8_t* px;
  int stride;      // pixels per row of px
  int ox, oy;      // region origin in full-image coordinates
  int rw, rh;      // region size
  int w, h;        // full image size
};

ImgView fullView(const Img& img);

// Fills [x0,x1)×[y0,y1) (clipped) with c, drawn Over.
void fillRect(Canvas& c, long long x0, long long y0, long long x1, long long y1, Rgba col);

// Draws the view's region Over the canvas with the full image's top-left at (x, y).
void blitImage(Canvas& c, const ImgView& v, long long x, long long y);

// Geometry of rotateImageCW: bounding box (size parity kept with the source)
// and the rotation's cosine/sine.
struct RotGeom {
  int nw, nh;
  float ct, st;
};
RotGeom rotGeom(int w, int h, double deg);

// The source rectangle (full-image coordinates, clipped to the image) whose
// pixels the rotated image needs where it is visible on the canvas, with the
// rotated box's top-left at (ox, oy). Returns false if nothing is visible.
bool rotSourceRect(const Canvas& c, int w, int h, const RotGeom& g, long long ox, long long oy, int* rx,
                   int* ry, int* rw, int* rh);

// Draws the view rotated clockwise (bilinear, premultiplied) Over the canvas,
// the rotated box's top-left at (ox, oy).
void drawRotated(Canvas& c, const ImgView& v, const RotGeom& g, long long ox, long long oy);

// Catmull-Rom scaling of src to dw×dh like xdraw.CatmullRom.Scale into a fresh
// NRGBA image; computes only the region [rx,rx+rw)×[ry,ry+rh) of the result
// into out (rw*rh*4 bytes, RGBA8888 non-premultiplied). Scratch memory comes
// from the budget.
void scaleCatmullRom(ZzaBudget* b, const Img& src, int dw, int dh, int rx, int ry, int rw, int rh,
                     uint8_t* out);

// copy layer: draw.Draw(dst, Rect(x,y,x+sw,y+sh), dst.SubImage(src rect), src, Src).
void copyRegion(Canvas& c, long long sx, long long sy, long long sw, long long sh, long long x, long long y);

}  // namespace zza
