#include "canvas.h"

#include <algorithm>
#include <cmath>
#include <cstring>

namespace zza {

namespace {

const double kPi = 3.14159265358979323846;

long long clampLL(long long v, long long lo, long long hi) { return v < lo ? lo : v > hi ? hi : v; }

// One contribution of a source column/row to a destination column/row.
struct Contrib {
  int32_t coord;
  float w;
};

// Port of x/image/draw newDistrib for the destination coordinates
// [d0, d0+n) of a dw-from-sw mapping with the Catmull-Rom kernel (support 2).
// Weights are computed in double per column/row (not per pixel) like Go.
// Contribution k of coordinate d0+i is cs[start[i] .. start[i+1]); inv[i] is
// the inverse total weight.
void distrib(int dw, int sw, int d0, int n, ZzaVec<Contrib>& cs, ZzaVec<uint32_t>& start, ZzaVec<float>& inv) {
  double scale = (double)sw / (double)dw;
  double halfWidth = 2, argScale = 1;
  if (scale > 1) {
    halfWidth *= scale;
    argScale = 1 / scale;
  }
  start.resize((size_t)n + 1);
  inv.resize((size_t)n);
  for (int k = 0; k < n; k++) {
    start[k] = (uint32_t)cs.size();
    double center = ((double)(d0 + k) + 0.5) * scale - 0.5;
    long long i = (long long)std::floor(center - halfWidth);
    if (i < 0) i = 0;
    long long j = (long long)std::ceil(center + halfWidth);
    if (j > sw) j = sw;
    if (j < i) j = i;
    double total = 0;
    for (long long coord = i; coord < j; coord++) {
      double t = std::fabs((center - (double)coord) * argScale);
      if (t >= 2) continue;
      double w = t < 1 ? (1.5 * t - 2.5) * t * t + 1 : ((-0.5 * t + 2.5) * t - 4) * t + 2;
      if (w == 0) continue;
      total += w;
      Contrib c;
      c.coord = (int32_t)coord;
      c.w = (float)w;
      cs.push_back(c);
    }
    inv[k] = total != 0 ? (float)(1 / total) : 0.0f;
  }
  start[n] = (uint32_t)cs.size();
}

// ftou of x/image/draw: [0,1] -> [0,0xffff].
inline uint32_t ftou(float f) {
  int32_t i = (int32_t)(65535.0f * f + 0.5f);
  if (i > 0xffff) return 0xffff;
  return i > 0 ? (uint32_t)i : 0;
}

}  // namespace

ImgView fullView(const Img& img) {
  ImgView v;
  v.px = img.px.data();
  v.stride = img.w;
  v.ox = 0;
  v.oy = 0;
  v.rw = img.w;
  v.rh = img.h;
  v.w = img.w;
  v.h = img.h;
  return v;
}

void fillRect(Canvas& c, long long x0, long long y0, long long x1, long long y1, Rgba col) {
  x0 = clampLL(x0, 0, c.w);
  x1 = clampLL(x1, 0, c.w);
  y0 = clampLL(y0, 0, c.h);
  y1 = clampLL(y1, 0, c.h);
  if (x0 >= x1 || y0 >= y1 || col.a == 0) return;
  charge(c.work, (uint64_t)((x1 - x0) * (y1 - y0)));
  for (long long y = y0; y < y1; y++) {
    uint8_t* d = c.at((int)x0, (int)y);
    for (long long x = x0; x < x1; x++, d += 3) blendPx(d, col.r, col.g, col.b, col.a);
  }
}

void blitImage(Canvas& c, const ImgView& v, long long x, long long y) {
  long long X0 = x + v.ox, Y0 = y + v.oy;  // canvas position of the region's top-left
  long long xa = clampLL(-X0, 0, v.rw), xb = clampLL(c.w - X0, 0, v.rw);
  long long ya = clampLL(-Y0, 0, v.rh), yb = clampLL(c.h - Y0, 0, v.rh);
  if (xa >= xb || ya >= yb) return;
  charge(c.work, (uint64_t)((xb - xa) * (yb - ya)));
  for (long long yy = ya; yy < yb; yy++) {
    const uint8_t* s = v.px + 4 * ((size_t)yy * v.stride + (size_t)xa);
    uint8_t* d = c.at((int)(X0 + xa), (int)(Y0 + yy));
    for (long long xx = xa; xx < xb; xx++, s += 4, d += 3) {
      uint8_t a = s[3];
      if (a == 0) continue;
      if (a == 255) {
        d[0] = s[0];
        d[1] = s[1];
        d[2] = s[2];
        continue;
      }
      blendPx(d, s[0], s[1], s[2], a);
    }
  }
}

RotGeom rotGeom(int w, int h, double deg) {
  double T = deg * kPi / 180.0;
  double ct = std::cos(T), st = std::sin(T);
  RotGeom g;
  // w, h <= 65535 * 16384 (renderImage), so the box fits; clamped as a guard.
  g.nw = (int)std::min(std::ceil((double)w * std::fabs(ct) + (double)h * std::fabs(st)), (double)(1 << 30));
  g.nh = (int)std::min(std::ceil((double)w * std::fabs(st) + (double)h * std::fabs(ct)), (double)(1 << 30));
  // Keep the size difference even (rotateImageCW), so the axis stays on the grid.
  if ((g.nw - w) % 2 != 0) g.nw++;
  if ((g.nh - h) % 2 != 0) g.nh++;
  g.ct = (float)ct;
  g.st = (float)st;
  return g;
}

bool rotSourceRect(const Canvas& c, int w, int h, const RotGeom& g, long long ox, long long oy, int* rx,
                   int* ry, int* rw, int* rh) {
  long long dxa = clampLL(-ox, 0, g.nw), dxb = clampLL(c.w - ox, 0, g.nw);
  long long dya = clampLL(-oy, 0, g.nh), dyb = clampLL(c.h - oy, 0, g.nh);
  if (dxa >= dxb || dya >= dyb) return false;
  double dcx = g.nw / 2.0, dcy = g.nh / 2.0, scx = w / 2.0, scy = h / 2.0;
  double minX = 1e300, maxX = -1e300, minY = 1e300, maxY = -1e300;
  const long long xs[2] = {dxa, dxb}, ys[2] = {dya, dyb};
  for (long long px : xs) {
    for (long long py : ys) {
      double qx = (double)px - dcx, qy = (double)py - dcy;
      double sx = scx + g.ct * qx + g.st * qy, sy = scy - g.st * qx + g.ct * qy;
      minX = std::min(minX, sx);
      maxX = std::max(maxX, sx);
      minY = std::min(minY, sy);
      maxY = std::max(maxY, sy);
    }
  }
  long long x0 = clampLL((long long)std::floor(minX) - 2, 0, w), x1 = clampLL((long long)std::ceil(maxX) + 2, 0, w);
  long long y0 = clampLL((long long)std::floor(minY) - 2, 0, h), y1 = clampLL((long long)std::ceil(maxY) + 2, 0, h);
  if (x0 >= x1 || y0 >= y1) return false;
  *rx = (int)x0;
  *ry = (int)y0;
  *rw = (int)(x1 - x0);
  *rh = (int)(y1 - y0);
  return true;
}

void drawRotated(Canvas& c, const ImgView& v, const RotGeom& g, long long ox, long long oy) {
  long long dxa = clampLL(-ox, 0, g.nw), dxb = clampLL(c.w - ox, 0, g.nw);
  long long dya = clampLL(-oy, 0, g.nh), dyb = clampLL(c.h - oy, 0, g.nh);
  if (dxa >= dxb || dya >= dyb || v.rw <= 0 || v.rh <= 0) return;
  charge(c.work, (uint64_t)((dxb - dxa) * (dyb - dya)));
  const float ct = g.ct, st = g.st;
  const float scx = v.w / 2.0f, scy = v.h / 2.0f, dcx = g.nw / 2.0f, dcy = g.nh / 2.0f;
  const float fw = (float)v.w, fh = (float)v.h;
  const int rx1 = v.ox + v.rw - 1, ry1 = v.oy + v.rh - 1;
  auto pix = [&](int x, int y) -> const uint8_t* {
    // Clamp into the region (it covers every tap by construction; this is a guard).
    x = x < v.ox ? v.ox : x > rx1 ? rx1 : x;
    y = y < v.oy ? v.oy : y > ry1 ? ry1 : y;
    return v.px + 4 * ((size_t)(y - v.oy) * v.stride + (size_t)(x - v.ox));
  };
  for (long long dy = dya; dy < dyb; dy++) {
    float py = (float)dy + 0.5f - dcy;
    float px = (float)dxa + 0.5f - dcx;
    float sx = scx + ct * px + st * py;  // stepped by (ct, -st) per pixel
    float sy = scy - st * px + ct * py;
    uint8_t* d = c.at((int)(ox + dxa), (int)(oy + dy));
    for (long long dx = dxa; dx < dxb; dx++, sx += ct, sy -= st, d += 3) {
      if (!(sx >= 0 && sx < fw && sy >= 0 && sy < fh)) continue;
      float fx = sx - 0.5f, fy = sy - 0.5f;
      int x0 = (int)fx, y0 = (int)fy;  // truncation like x/image/draw
      int wx = (int)((fx - (float)x0) * 256.0f), wy = (int)((fy - (float)y0) * 256.0f);
      int x1 = x0 + 1, y1 = y0 + 1;
      if (fx < 0) {
        x0 = x1 = 0;
        wx = 0;
      } else if (x1 >= v.w) {
        x1 = x0;
        wx = 0;
      }
      if (fy < 0) {
        y0 = y1 = 0;
        wy = 0;
      } else if (y1 >= v.h) {
        y1 = y0;
        wy = 0;
      }
      const uint8_t* p[4] = {pix(x0, y0), pix(x1, y0), pix(x0, y1), pix(x1, y1)};
      if (!(p[0][3] | p[1][3] | p[2][3] | p[3][3])) continue;  // fully transparent
      const uint32_t w[4] = {(uint32_t)((256 - wx) * (256 - wy)), (uint32_t)(wx * (256 - wy)),
                             (uint32_t)((256 - wx) * wy), (uint32_t)(wx * wy)};
      uint32_t aa = 0, ar = 0, ag = 0, ab = 0;  // premultiplied; weights sum to 65536
      for (int k = 0; k < 4; k++) {
        uint32_t wa = w[k] * p[k][3];
        aa += wa;
        ar += p[k][0] * wa;
        ag += p[k][1] * wa;
        ab += p[k][2] * wa;
      }
      if (!aa) continue;
      blendPx(d, (ar + aa / 2) / aa, (ag + aa / 2) / aa, (ab + aa / 2) / aa, (aa + 32768) >> 16);
    }
  }
}

void scaleCatmullRom(ZzaBudget* b, Work* work, const Img& src, int dw, int dh, int rx, int ry, int rw, int rh,
                     uint8_t* out) {
  if (rw <= 0 || rh <= 0 || src.w == 0 || src.h == 0) return;
  charge(work, (uint64_t)rw + (uint64_t)rh);
  ZzaVec<Contrib> hc{ZzaAlloc<Contrib>(b)}, vc{ZzaAlloc<Contrib>(b)};
  ZzaVec<uint32_t> hs{ZzaAlloc<uint32_t>(b)}, vs{ZzaAlloc<uint32_t>(b)};
  ZzaVec<float> hinv{ZzaAlloc<float>(b)}, vinv{ZzaAlloc<float>(b)};
  distrib(dw, src.w, rx, rw, hc, hs, hinv);
  distrib(dh, src.h, ry, rh, vc, vs, vinv);
  charge(work, (uint64_t)hc.size() + vc.size());

  // The source rows of one destination row form a window that only moves
  // down; K rows of horizontally scaled values are kept in a ring.
  int K = 1;
  for (int y = 0; y < rh; y++)
    if (vs[y] < vs[y + 1]) K = std::max(K, vc[vs[y + 1] - 1].coord - vc[vs[y]].coord + 1);
  const size_t rowLen = (size_t)rw * 4;
  ZzaVec<float> ring{ZzaAlloc<float>(b)};
  ring.resize((size_t)K * rowLen);
  ZzaVec<int32_t> slotRow{ZzaAlloc<int32_t>(b)};
  slotRow.assign((size_t)K, -1);

  auto hrow = [&](int r) -> const float* {
    float* t = ring.data() + (size_t)(r % K) * rowLen;
    if (slotRow[r % K] == r) return t;
    slotRow[r % K] = r;
    charge(work, (uint64_t)hc.size() + (uint64_t)rw);  // every tap of the row
    const uint8_t* row = src.px.data() + (size_t)r * src.w * 4;
    for (int k = 0; k < rw; k++) {
      float pr = 0, pg = 0, pb = 0, pa = 0;
      for (uint32_t ci = hs[k]; ci < hs[k + 1]; ci++) {
        const uint8_t* s = row + (size_t)hc[ci].coord * 4;
        uint32_t pau = (uint32_t)s[3] * 0x101;  // 16-bit premultiplied like x/image/draw
        float w = hc[ci].w;
        pr += (float)(s[0] * pau / 0xff) * w;
        pg += (float)(s[1] * pau / 0xff) * w;
        pb += (float)(s[2] * pau / 0xff) * w;
        pa += (float)pau * w;
      }
      float f = hinv[k] / 65535.0f;
      t[4 * k] = pr * f;
      t[4 * k + 1] = pg * f;
      t[4 * k + 2] = pb * f;
      t[4 * k + 3] = pa * f;
    }
    return t;
  };

  for (int y = 0; y < rh; y++) {
    uint8_t* o = out + (size_t)y * rowLen;
    charge(work, (uint64_t)(vs[y + 1] - vs[y] + 1) * (uint64_t)rw);
    memset(o, 0, rowLen);
    for (uint32_t ci = vs[y]; ci < vs[y + 1]; ci++) hrow(vc[ci].coord);  // fill the window first
    for (int k = 0; k < rw; k++) {
      float pr = 0, pg = 0, pb = 0, pa = 0;
      for (uint32_t ci = vs[y]; ci < vs[y + 1]; ci++) {
        const float* t = ring.data() + (size_t)(vc[ci].coord % K) * rowLen + 4 * (size_t)k;
        float w = vc[ci].w;
        pr += t[0] * w;
        pg += t[1] * w;
        pb += t[2] * w;
        pa += t[3] * w;
      }
      if (pr > pa) pr = pa;
      if (pg > pa) pg = pa;
      if (pb > pa) pb = pa;
      float iv = vinv[y];
      uint32_t r = ftou(pr * iv), g = ftou(pg * iv), bl = ftou(pb * iv), a = ftou(pa * iv);
      // color.RGBA64 -> NRGBA (image.NRGBA.SetRGBA64).
      if (a != 0 && a != 0xffff) {
        r = r * 0xffff / a;
        g = g * 0xffff / a;
        bl = bl * 0xffff / a;
      }
      o[4 * k] = (uint8_t)(r >> 8);
      o[4 * k + 1] = (uint8_t)(g >> 8);
      o[4 * k + 2] = (uint8_t)(bl >> 8);
      o[4 * k + 3] = (uint8_t)(a >> 8);
    }
  }
}

void copyRegion(Canvas& c, long long sx, long long sy, long long sw, long long sh, long long x, long long y) {
  // src = dst.SubImage(src rect): clipped to the canvas first.
  long long s0x = clampLL(sx, 0, c.w), s1x = clampLL(sx + sw, 0, c.w);
  long long s0y = clampLL(sy, 0, c.h), s1y = clampLL(sy + sh, 0, c.h);
  if (s0x >= s1x || s0y >= s1y) return;
  // draw.Draw clip: r ∩ dst bounds ∩ (src bounds moved to the destination).
  long long dxo = x - sx, dyo = y - sy;
  long long r0x = std::max(std::max(x, 0LL), s0x + dxo), r1x = std::min(std::min(x + sw, (long long)c.w), s1x + dxo);
  long long r0y = std::max(std::max(y, 0LL), s0y + dyo), r1y = std::min(std::min(y + sh, (long long)c.h), s1y + dyo);
  if (r0x >= r1x || r0y >= r1y) return;
  size_t n = (size_t)(r1x - r0x) * 3;
  long long rows = r1y - r0y;
  charge(c.work, (uint64_t)((r1x - r0x) * rows));
  bool up = dyo > 0;  // destination below the source: rows bottom-up (drawCopySrc)
  for (long long k = 0; k < rows; k++) {
    long long ry = up ? r1y - 1 - k : r0y + k;
    memmove(c.at((int)r0x, (int)ry), c.at((int)(r0x - dxo), (int)(ry - dyo)), n);
  }
}

}  // namespace zza
