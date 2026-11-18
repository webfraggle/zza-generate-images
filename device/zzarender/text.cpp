#include "text.h"

#include <algorithm>
#include <cmath>
#include <string_view>

#include "utf8.h"

namespace zza {

namespace {

// Bound for line heights, widths and positions so that line arithmetic cannot
// overflow (far beyond any canvas).
constexpr long long kMaxCoord = 1LL << 24;

long long clampSpan(long long v, long long lo, long long hi) { return v < lo ? lo : v > hi ? hi : v; }

// Range-checked double -> integer: NaN gives 0, the rest is clamped to ±lim.
long long toLL(double v, long long lim) {
  if (!(v == v)) return 0;
  if (v <= -(double)lim) return -lim;
  if (v >= (double)lim) return lim;
  return (long long)v;
}

long long floorDiv64(long long v) { return v >= 0 ? v / 64 : -((-v + 63) / 64); }

// Round of a 26.6 value (fixed.Int26_6.Round).
long long round26(long long v) { return floorDiv64(v + 32); }

// The glyph drawn for Latin-1 byte c and the code point it stands for: control
// characters and characters missing from the set become '?'.
const Glyph* glyphFor(const Font& f, uint8_t c, uint8_t* mapped) {
  const Glyph* g = c >= 0x20 ? f.find(c) : nullptr;
  if (!g) {
    c = '?';
    g = f.find('?');
  }
  *mapped = c;
  return g;
}

// Natural pen advance of a string, with its first and last mapped code
// (-1 if empty), so that measures can be concatenated.
struct Meas {
  long long pen;
  int first, last;
};

Meas emptyMeas() {
  Meas m;
  m.pen = 0;
  m.first = m.last = -1;
  return m;
}

Meas measure(const Font& f, std::string_view s) {
  Meas m = emptyMeas();
  for (unsigned char ch : s) {
    uint8_t c;
    const Glyph* g = glyphFor(f, ch, &c);
    if (m.last >= 0) m.pen += f.kernFor((unsigned)m.last, c);
    if (g) m.pen += g->adv;
    if (m.first < 0) m.first = c;
    m.last = c;
  }
  return m;
}

// Measure of the concatenation a+b.
Meas concat(const Font& f, const Meas& a, const Meas& b) {
  if (a.first < 0) return b;
  if (b.first < 0) return a;
  Meas m;
  m.pen = a.pen + f.kernFor((unsigned)a.last, (unsigned)b.first) + b.pen;
  m.first = a.first;
  m.last = b.last;
  return m;
}

long long natWidth(const Meas& m) { return clampSpan(round26(m.pen), -kMaxCoord, kMaxCoord); }

// scaledWidth: on-canvas width of a natural width; scale 0 = inactive.
long long scaledWidth(long long w, double scale) {
  return scale <= 0 ? w : toLL(std::ceil((double)w * scale), kMaxCoord);
}

// The text as Latin-1 (one byte per rune, '?' above U+00FF and for invalid
// bytes) with a flag per byte whether the rune was a Unicode space, so that
// splitting and trimming keep Go's UTF-8 semantics. Budget memory.
struct Latin1Text {
  ZzaBytes s;
  ZzaBytes space;
  explicit Latin1Text(ZzaBudget* b) : s(ZzaAlloc<uint8_t>(b)), space(ZzaAlloc<uint8_t>(b)) {}
  std::string_view view(size_t off, size_t len) const {
    return std::string_view(reinterpret_cast<const char*>(s.data()) + off, len);
  }
};

void toLatin1(std::string_view in, Latin1Text& t) {
  t.s.reserve(in.size());
  t.space.reserve(in.size());
  size_t i = 0;
  while (i < in.size()) {
    uint32_t r;
    i += decodeRune(in.data() + i, in.size() - i, &r);
    t.s.push_back(r <= 0xFF ? (uint8_t)r : (uint8_t)'?');
    t.space.push_back(isSpaceRune(r) ? 1 : 0);
  }
}

// A line of the layout: bytes [off, off+len) of a buffer, with its measure.
struct LineRef {
  uint32_t off, len;
  Meas m;
};

// wrapText: break at spaces, then after hyphens, then mid-word (Go semantics,
// including its quirks: a word "a-" yields an empty token, the mid-word break
// can emit an empty line). Lines are written to buf (joined tokens).
void wrapText(ZzaBudget* b, const Font& f, const Latin1Text& t, long long maxWidth, ZzaBytes& buf,
              ZzaVec<LineRef>& lines) {
  struct Token {
    uint32_t off, len;
    Meas m;
    bool spaceBefore;
  };
  ZzaVec<Token> toks{ZzaAlloc<Token>(b)};
  const size_t n = t.s.size();
  size_t i = 0;
  int word = 0;
  while (i < n) {  // strings.Fields
    if (t.space[i]) {
      i++;
      continue;
    }
    size_t st = i;
    while (i < n && !t.space[i]) i++;
    bool first = true;
    for (size_t ps = st;;) {  // strings.SplitAfter(word, "-")
      size_t h = ps;
      while (h < i && t.s[h] != '-') h++;
      size_t pe = h < i ? h + 1 : i;
      Token tk;
      tk.off = (uint32_t)ps;
      tk.len = (uint32_t)(pe - ps);
      tk.m = measure(f, t.view(ps, pe - ps));
      tk.spaceBefore = word > 0 && first;
      toks.push_back(tk);
      first = false;
      if (h >= i) break;
      ps = pe;
    }
    word++;
  }
  if (toks.empty()) return;

  buf.reserve(n + 1);
  const Meas space = measure(f, " ");
  LineRef cur;
  cur.off = 0;
  cur.len = 0;
  cur.m = emptyMeas();
  auto flush = [&]() {
    lines.push_back(cur);
    cur.off = (uint32_t)buf.size();
    cur.len = 0;
    cur.m = emptyMeas();
  };
  auto append = [&](const uint8_t* p, size_t len) {
    buf.insert(buf.end(), p, p + len);
    cur.len += (uint32_t)len;
  };
  for (const Token& tk : toks) {
    bool sep = cur.len > 0 && tk.spaceBefore;
    Meas cand = concat(f, sep ? concat(f, cur.m, space) : cur.m, tk.m);
    if (natWidth(cand) <= maxWidth) {
      if (sep) append(reinterpret_cast<const uint8_t*>(" "), 1);
      append(t.s.data() + tk.off, tk.len);
      cur.m = cand;
    } else if (cur.len > 0) {
      flush();
      append(t.s.data() + tk.off, tk.len);
      cur.m = tk.m;
    } else {
      // Single token wider than maxWidth: break between characters.
      for (uint32_t k = 0; k < tk.len; k++) {
        const uint8_t* ch = t.s.data() + tk.off + k;
        Meas cm = measure(f, std::string_view(reinterpret_cast<const char*>(ch), 1));
        Meas c2 = concat(f, cur.m, cm);
        if (natWidth(c2) <= maxWidth) {
          append(ch, 1);
          cur.m = c2;
        } else {
          flush();
          append(ch, 1);
          cur.m = cm;
        }
      }
    }
  }
  if (cur.len > 0) lines.push_back(cur);
}

// Natural (unscaled) value for a metric stored as round(natural * sy): the
// candidate closest to stored / sy among those that map back to it. sy is
// in [0.05, 8] (validated by the layer), so the search covers a few values.
long long naturalHeight(int stored, double sy) {
  long long best = toLL(std::round(stored / sy), kMaxCoord);
  double bestD = 1e300;
  long long lo = toLL(std::floor((stored - 0.5) / sy), kMaxCoord) - 1;
  long long hi = toLL(std::ceil((stored + 0.5) / sy), kMaxCoord) + 1;
  if (hi - lo > 64) return best;  // guard; cannot happen for a valid sy
  for (long long v = lo; v <= hi; v++) {
    if (std::round((double)v * sy) != (double)stored) continue;
    double d = std::fabs((double)v - stored / sy);
    if (d < bestD) {
      bestD = d;
      best = v;
    }
  }
  return best;
}

// visibleRun: the part of line whose glyphs, drawn from x and scaled
// horizontally by scale, can overlap [minX, maxX), and the x it starts at.
std::string_view visibleRun(const Font& f, std::string_view line, long long* x, double scale, long long minX,
                            long long maxX) {
  double x0 = (double)*x;
  auto pos = [&](long long adv) { return x0 + (double)adv / 64 * scale; };
  long long adv = 0, startAdv = 0;
  long long start = -1, end = 0;
  int prev = -1;
  for (size_t i = 0; i < line.size(); i++) {
    uint8_t m;
    const Glyph* g = glyphFor(f, (uint8_t)line[i], &m);
    if (prev >= 0) adv += f.kernFor((unsigned)prev, m);
    long long left = adv;
    if (g) adv += g->adv;
    prev = m;
    if (pos(adv) < (double)minX) continue;
    if (pos(left) >= (double)maxX) break;
    if (start < 0) {
      start = (long long)i;
      startAdv = left;
    }
    end = (long long)i + 1;
  }
  if (start < 0) return std::string_view();
  *x = toLL(std::round(pos(startAdv)), 1LL << 40);
  return line.substr((size_t)start, (size_t)(end - start));
}

// Draws a Latin-1 line with the pen origin at (x, baselineY) like
// drawTextLine. Unscaled: glyph at x + round(pen/64). Scaled (scaleX or
// scaleY active): Go draws the line offscreen with a margin pad = ascent and
// resamples it to ceil(size * scale); glyph positions follow that mapping so
// the pre-scaled masks land where Go's resampled glyphs do. Glyphs off the
// canvas are skipped.
void drawLine(Canvas& c, const Font& f, std::string_view s, long long x, long long baseY, Rgba col, double scaleX,
              double scaleY) {
  charge(c.work, s.size() + 1);  // layout of every glyph, drawn or not
  bool sxA = scaleX > 0 && scaleX != 1, syA = scaleY > 0 && scaleY != 1;
  bool scaled = sxA || syA;
  double ox = 0, rx = 1;  // glyph x = ox + (pen/64) * rx
  if (scaled) {
    double sx = sxA ? scaleX : 1, sy = syA ? scaleY : 1;
    long long A = syA ? naturalHeight(f.ascent, sy) : f.ascent;  // natural ascent = pad
    long long D = f.descent;
    long long W = natWidth(measure(f, s));
    if ((double)(W + 2 * A) * sx > (double)(2 * c.w + 4 * A)) {
      s = visibleRun(f, s, &x, sx, -2 * A, c.w + 2 * A);
      W = natWidth(measure(f, s));
    }
    if (W <= 0) return;
    double offW = (double)(W + 2 * A), offH = (double)(A + D + 2 * A);
    if (offW <= 0) return;  // odd metrics
    double dw = std::ceil(offW * sx), dh = std::ceil(offH * sy);
    double dx = (double)x - std::round((double)A * sx);
    double dy = (double)baseY - std::round((double)(2 * A) * sy);
    rx = dw / offW;
    ox = dx + (double)A * rx;
    if (offH > 0) baseY = toLL(std::floor(dy + (double)(2 * A) * (dh / offH) + 0.5), 1LL << 40);
  }
  const float frx = (float)rx, fox = (float)ox;
  long long pen = 0;
  int prev = -1;
  for (unsigned char ch : s) {
    uint8_t m;
    const Glyph* g = glyphFor(f, ch, &m);
    if (prev >= 0) pen += f.kernFor((unsigned)prev, m);
    prev = m;
    if (!g) continue;
    long long gx = scaled ? toLL(std::floor(fox + (float)pen / 64.0f * frx + 0.5f), 1LL << 40) : x + round26(pen);
    pen += g->adv;
    if (g->w == 0 || g->h == 0) continue;
    long long X0 = gx + g->bx, Y0 = baseY + g->by;
    if (X0 >= c.w || Y0 >= c.h || X0 + g->w <= 0 || Y0 + g->h <= 0) continue;
    int xa = X0 < 0 ? (int)-X0 : 0, xb = X0 + g->w > c.w ? (int)(c.w - X0) : g->w;
    int ya = Y0 < 0 ? (int)-Y0 : 0, yb = Y0 + g->h > c.h ? (int)(c.h - Y0) : g->h;
    charge(c.work, (uint64_t)(xb - xa) * (uint64_t)(yb - ya));
    const uint8_t* mask = f.mask(*g);
    for (int yy = ya; yy < yb; yy++) {
      const uint8_t* mp = mask + (size_t)yy * g->w + xa;
      uint8_t* d = c.at((int)(X0 + xa), (int)(Y0 + yy));
      for (int xx = xa; xx < xb; xx++, mp++, d += 3) {
        uint32_t a = col.a == 255 ? *mp : div255((uint32_t)*mp * col.a);
        blendPx(d, col.r, col.g, col.b, a);
      }
    }
  }
}

// Trimmed Latin-1 range [*a, *b) by the space flags (strings.TrimSpace).
void trimRange(const Latin1Text& t, size_t* a, size_t* b) {
  while (*a < *b && t.space[*a]) (*a)++;
  while (*b > *a && t.space[*b - 1]) (*b)--;
}

}  // namespace

std::string capText(std::string s) {
  if (s.size() <= kMaxTextRunes) return s;  // bytes >= runes
  size_t i = 0, n = 0;
  while (i < s.size()) {
    if (n == kMaxTextRunes) {
      s.resize(i);
      return s;
    }
    uint32_t r;
    i += decodeRune(s.data() + i, s.size() - i, &r);
    n++;
  }
  return s;
}

bool validScale(double s) { return s == 0 || (s >= kMinScale && s <= kMaxScale); }

int drawTextLayer(ZzaBudget* b, Canvas& c, const Font& f, const TextParams& p, const std::string& text, Rgba col) {
  long long ascent = f.ascent;
  long long lineHeight = f.height;
  bool lhActive = p.lineHeight > 0 && p.lineHeight != 1;  // false for NaN
  bool syActive = p.scaleY > 0 && p.scaleY != 1;
  // Go: line_height applies to the natural height, then scale_y; the file
  // holds the scaled height (doc.go).
  if (lhActive && syActive) {
    double nat = std::round((double)naturalHeight(f.height, p.scaleY) * p.lineHeight);
    lineHeight = toLL(std::round(nat * p.scaleY), kMaxCoord);
  } else if (lhActive) {
    lineHeight = toLL(std::round((double)lineHeight * p.lineHeight), kMaxCoord);
  }
  double scale = p.scaleX > 0 && p.scaleX != 1 ? p.scaleX : 0;
  bool baseline = p.valign == kValignBaseline;

  charge(c.work, text.size() + 1);  // conversion, measuring and wrapping (linear)
  Latin1Text t(b);
  toLatin1(text, t);

  if (p.fitItems) {
    // strings.Split on the UTF-8 text; rune counts map parts to Latin-1 offsets.
    long long baseY = baseline ? p.y : p.y + ascent;
    long long xpos = 0;
    const std::string& sep = p.fitSplitBy;
    size_t pos = 0, runeAt = 0;  // UTF-8 byte offset and its rune index
    for (bool last = false; !last;) {
      size_t h = sep.empty() ? std::string::npos : text.find(sep, pos);
      last = h == std::string::npos;
      size_t endByte = last ? text.size() : h;
      size_t a = runeAt, k = pos;
      while (k < endByte) {
        uint32_t r;
        k += decodeRune(text.data() + k, text.size() - k, &r);
        runeAt++;
      }
      size_t e = runeAt;
      if (!last) {  // skip the separator's runes
        for (k = h; k < h + sep.size();) {
          uint32_t r;
          k += decodeRune(text.data() + k, text.size() - k, &r);
          runeAt++;
        }
        pos = h + sep.size();
      }
      trimRange(t, &a, &e);
      if (a == e) continue;
      std::string_view item = t.view(a, e - a);
      long long w = scaledWidth(natWidth(measure(f, item)), scale);
      if (p.maxWidth <= 0 || xpos + w < p.maxWidth) drawLine(c, f, item, p.x + xpos, baseY, col, p.scaleX, p.scaleY);
      xpos += w + p.fitGap;
    }
    return 0;
  }

  long long wrapWidth = p.maxWidth;
  if (p.width > 0 && wrapWidth == 0) wrapWidth = p.width;
  ZzaBytes buf{ZzaAlloc<uint8_t>(b)};
  ZzaVec<LineRef> lines{ZzaAlloc<LineRef>(b)};
  const uint8_t* src;  // lines index into src
  if (wrapWidth > 0) {
    if (scale > 0) wrapWidth = toLL((double)wrapWidth / scale, 1LL << 40);  // int(float64/scale)
    wrapText(b, f, t, wrapWidth, buf, lines);
    src = buf.data();
  } else {
    LineRef l;
    l.off = 0;
    l.len = (uint32_t)t.s.size();
    l.m = measure(f, t.view(0, t.s.size()));
    lines.push_back(l);
    src = t.s.data();
  }
  long long widest = 0;
  for (const LineRef& l : lines) widest = std::max(widest, scaledWidth(natWidth(l.m), scale));
  if (lines.empty()) return (int)widest;

  long long startY = p.y;
  if (baseline) {
    startY = p.y - ascent;
  } else if (p.height > 0) {
    long long total = (long long)lines.size() * lineHeight;
    if (p.valign == kValignMiddle) startY = p.y + (p.height - total) / 2;
    else if (p.valign == kValignBottom) startY = p.y + p.height - total;
    if (startY < p.y) startY = p.y;
  }

  for (size_t i = 0; i < lines.size(); i++) {
    long long y = startY + ascent + (long long)i * lineHeight;
    if (y - ascent >= c.h) break;
    if (y + lineHeight < 0) continue;
    long long x = p.x;
    if (p.align != kAlignLeft) {
      long long lw = scaledWidth(natWidth(lines[i].m), scale);
      if (p.align == kAlignCenter) x = p.width > 0 ? p.x + (p.width - lw) / 2 : p.x - lw / 2;
      else x = p.width > 0 ? p.x + p.width - lw : p.x - lw;
    }
    std::string_view s(reinterpret_cast<const char*>(src) + lines[i].off, lines[i].len);
    drawLine(c, f, s, x, y, col, p.scaleX, p.scaleY);
  }
  return (int)widest;
}

}  // namespace zza
