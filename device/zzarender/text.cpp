#include "text.h"

#include <cmath>
#include <string_view>
#include <vector>

#include "utf8.h"

namespace zza {

namespace {

// Bound for line heights and positions so that line arithmetic cannot overflow.
constexpr long long kMaxCoord = 1LL << 24;

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

Meas measure(const Font& f, std::string_view s) {
  Meas m;
  m.pen = 0;
  m.first = m.last = -1;
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

int natWidth(const Meas& m) {
  long long w = round26(m.pen);
  return (int)(w > kMaxCoord ? kMaxCoord : w < -kMaxCoord ? -kMaxCoord : w);
}

// scaledWidth: on-canvas width of a natural width; scale 0 = inactive.
int scaledWidth(int w, double scale) { return scale <= 0 ? w : (int)std::ceil((double)w * scale); }

struct Line {
  std::string s;  // Latin-1
  Meas m;
};

std::string toLatin1(std::string_view s) {
  std::string out;
  utf8ToLatin1(s, out, false);
  return out;
}

// wrapText: break at spaces, then after hyphens, then mid-word (Go semantics,
// including its quirks). Splitting happens on the UTF-8 text, measuring on Latin-1.
void wrapText(const Font& f, const std::string& text, int maxWidth, std::vector<Line>& lines) {
  struct Token {
    std::string s;
    Meas m;
    bool spaceBefore;
  };
  std::vector<Token> toks;
  size_t i = 0, n = text.size();
  int word = 0;
  while (i < n) {
    uint32_t r;
    int k = decodeRune(text.data() + i, n - i, &r);
    if (isSpaceRune(r)) {
      i += k;
      continue;
    }
    size_t st = i;
    while (i < n) {
      k = decodeRune(text.data() + i, n - i, &r);
      if (isSpaceRune(r)) break;
      i += k;
    }
    std::string_view w(text.data() + st, i - st);
    size_t ps = 0;
    bool first = true;
    for (;;) {  // strings.SplitAfter(word, "-")
      size_t h = w.find('-', ps);
      size_t pe = h == std::string_view::npos ? w.size() : h + 1;
      Token t;
      t.s = toLatin1(w.substr(ps, pe - ps));
      t.m = measure(f, t.s);
      t.spaceBefore = word > 0 && first;
      toks.push_back(std::move(t));
      first = false;
      if (h == std::string_view::npos) break;
      ps = pe;
    }
    word++;
  }
  if (toks.empty()) return;

  const Meas space = measure(f, " ");
  Line cur;
  cur.m = measure(f, "");
  for (const Token& t : toks) {
    bool sep = !cur.s.empty() && t.spaceBefore;
    Meas cand = concat(f, sep ? concat(f, cur.m, space) : cur.m, t.m);
    if (natWidth(cand) <= maxWidth) {
      if (sep) cur.s += ' ';
      cur.s += t.s;
      cur.m = cand;
    } else if (!cur.s.empty()) {
      lines.push_back(std::move(cur));
      cur.s = t.s;
      cur.m = t.m;
    } else {
      // Single token wider than maxWidth: break between characters.
      for (char ch : t.s) {
        Meas cm = measure(f, std::string_view(&ch, 1));
        Meas c2 = concat(f, cur.m, cm);
        if (natWidth(c2) <= maxWidth) {
          cur.s += ch;
          cur.m = c2;
        } else {
          lines.push_back(std::move(cur));
          cur.s.assign(1, ch);
          cur.m = cm;
        }
      }
    }
  }
  if (!cur.s.empty()) lines.push_back(std::move(cur));
}

// Natural (unscaled) value for a metric stored as round(natural * sy): the
// candidate closest to stored / sy among those that map back to it.
int naturalHeight(int stored, double sy) {
  int best = (int)std::lround(stored / sy);
  double bestD = 1e300;
  long long lo = (long long)std::floor((stored - 0.5) / sy) - 1, hi = (long long)std::ceil((stored + 0.5) / sy) + 1;
  for (long long v = lo; v <= hi; v++) {
    if (std::round((double)v * sy) != (double)stored) continue;
    double d = std::fabs((double)v - stored / sy);
    if (d < bestD) {
      bestD = d;
      best = (int)v;
    }
  }
  return best;
}

// visibleRun: the part of line whose glyphs, drawn from x and scaled
// horizontally by scale, can overlap [minX, maxX), and the x it starts at.
std::string visibleRun(const Font& f, const std::string& line, long long* x, double scale, long long minX,
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
  if (start < 0) return std::string();
  *x = (long long)std::round(pos(startAdv));
  return line.substr((size_t)start, (size_t)(end - start));
}

// Draws a Latin-1 line with the pen origin at (x, baselineY) like
// drawTextLine. Unscaled: glyph at x + round(pen/64). Scaled (scaleX or
// scaleY active): Go draws the line offscreen with a margin pad = ascent and
// resamples it to ceil(size * scale); glyph positions follow that mapping so
// the pre-scaled masks land where Go's resampled glyphs do. Glyphs off the
// canvas are skipped.
void drawLine(Canvas& c, const Font& f, std::string s, long long x, long long baseY, Rgba col, double scaleX,
              double scaleY) {
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
    double dw = std::ceil(offW * sx), dh = std::ceil(offH * sy);
    double dx = (double)x - std::round((double)A * sx);
    double dy = (double)baseY - std::round((double)(2 * A) * sy);
    rx = dw / offW;
    ox = dx + (double)A * rx;
    baseY = (long long)std::floor(dy + (double)(2 * A) * (dh / offH) + 0.5);
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
    long long gx = scaled ? (long long)std::floor(fox + (float)pen / 64.0f * frx + 0.5f) : x + round26(pen);
    pen += g->adv;
    if (g->w == 0 || g->h == 0) continue;
    long long X0 = gx + g->bx, Y0 = baseY + g->by;
    if (X0 >= c.w || Y0 >= c.h || X0 + g->w <= 0 || Y0 + g->h <= 0) continue;
    int xa = X0 < 0 ? (int)-X0 : 0, xb = X0 + g->w > c.w ? (int)(c.w - X0) : g->w;
    int ya = Y0 < 0 ? (int)-Y0 : 0, yb = Y0 + g->h > c.h ? (int)(c.h - Y0) : g->h;
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

long long clampCoord(double v) {
  if (!(v > -(double)kMaxCoord)) return -kMaxCoord;
  if (!(v < (double)kMaxCoord)) return kMaxCoord;
  return (long long)v;
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

int drawTextLayer(Canvas& c, const Font& f, const TextParams& p, const std::string& text, Rgba col) {
  long long ascent = f.ascent;
  long long lineHeight = f.height;
  bool lhActive = p.lineHeight > 0 && p.lineHeight != 1;
  bool syActive = p.scaleY > 0 && p.scaleY != 1;
  // Go: line_height applies to the natural height, then scale_y; the file
  // holds the scaled height (doc.go).
  if (lhActive && syActive) {
    double nat = std::round(naturalHeight(f.height, p.scaleY) * p.lineHeight);
    lineHeight = clampCoord(std::round(nat * p.scaleY));
  } else if (lhActive) {
    lineHeight = clampCoord(std::round((double)lineHeight * p.lineHeight));
  }
  double scale = p.scaleX > 0 && p.scaleX != 1 ? p.scaleX : 0;
  bool baseline = p.valign == kValignBaseline;

  if (p.fitItems) {
    std::vector<std::string_view> parts;
    std::string_view t(text);
    if (p.fitSplitBy.empty()) {
      parts.push_back(t);
    } else {
      size_t st = 0;
      for (;;) {
        size_t h = t.find(p.fitSplitBy, st);
        if (h == std::string_view::npos) break;
        parts.push_back(t.substr(st, h - st));
        st = h + p.fitSplitBy.size();
      }
      parts.push_back(t.substr(st));
    }
    long long baseY = baseline ? p.y : p.y + ascent;
    long long xpos = 0;
    for (std::string_view part : parts) {
      part = trimSpace(part);
      if (part.empty()) continue;
      std::string s = toLatin1(part);
      int w = scaledWidth(natWidth(measure(f, s)), scale);
      if (p.maxWidth <= 0 || xpos + w < p.maxWidth) drawLine(c, f, s, p.x + xpos, baseY, col, p.scaleX, p.scaleY);
      xpos += (long long)w + p.fitGap;
    }
    return 0;
  }

  int wrapWidth = p.maxWidth;
  if (p.width > 0 && wrapWidth == 0) wrapWidth = p.width;
  std::vector<Line> lines;
  if (wrapWidth > 0) {
    if (scale > 0) wrapWidth = (int)((double)wrapWidth / scale);
    wrapText(f, text, wrapWidth, lines);
  } else {
    Line l;
    l.s = toLatin1(text);
    l.m = measure(f, l.s);
    lines.push_back(std::move(l));
  }
  int widest = 0;
  for (const Line& l : lines) {
    int w = scaledWidth(natWidth(l.m), scale);
    if (w > widest) widest = w;
  }
  if (lines.empty()) return widest;

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
    drawLine(c, f, lines[i].s, x, y, col, p.scaleX, p.scaleY);
  }
  return widest;
}

}  // namespace zza
