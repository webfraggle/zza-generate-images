// splitflap layers (port of internal/renderer/splitflap.go) and the flip
// drawing of animation.go.
#include "splitflap.h"

#include <algorithm>
#include <cstring>
#include <string>
#include <vector>

#include "render_impl.h"
#include "sha256.h"
#include "utf8.h"

namespace zza {

// ---- motion and drawing (animation.go) ----

int flipCount(const uint8_t seed[32], int layerIdx, int cellIdx, int minFlips, int maxFlips) {
  if (maxFlips <= minFlips) return minFlips;
  uint8_t idx[16];
  uint64_t a = (uint64_t)(int64_t)layerIdx, b = (uint64_t)(int64_t)cellIdx;
  for (int i = 0; i < 8; i++) {
    idx[i] = (uint8_t)(a >> (56 - 8 * i));
    idx[8 + i] = (uint8_t)(b >> (56 - 8 * i));
  }
  ZzaSha256 h;
  h.update(seed, 32);
  h.update(idx, sizeof idx);
  uint8_t sum[32];
  h.final(sum);
  uint64_t v = 0;
  for (int i = 0; i < 8; i++) v = v << 8 | sum[i];
  return minFlips + (int)(v % (uint64_t)(maxFlips - minFlips + 1));
}

int flipSeqEntry(int reelN, int targetIdx, int k, int j) {
  int start = targetIdx >= 0 ? targetIdx - k : k;
  return ((start + j) % reelN + reelN) % reelN;
}

void flipPhase(int phase, int phases, bool* upper, int* num, int* den) {
  int nUpper = (phases + 1) / 2;
  if (phase < nUpper) {
    *upper = true;
    *num = phase + 1;
    *den = nUpper + 1;
    return;
  }
  *upper = false;
  *num = phase - nUpper + 1;
  *den = phases - nUpper + 1;
}

int squashedHeight(int full, int num, int den) {
  // math.Round(full·num/den): half away from zero; the products are exact for
  // the denominators 2…4 that flipPhase yields, so integer rounding matches.
  int v = (int)((2LL * full * num + den) / (2LL * den));
  if (v < 1) v = 1;
  if (v > full) v = full;
  return v;
}

void shade(Canvas& c, int x0, int y0, int x1, int y1, uint32_t keep) {
  if (x0 < 0) x0 = 0;
  if (y0 < 0) y0 = 0;
  if (x1 > c.w) x1 = c.w;
  if (y1 > c.h) y1 = c.h;
  for (int y = y0; y < y1; y++) {
    uint8_t* p = c.at(x0, y);
    for (int n = (x1 - x0) * 3; n > 0; n--, p++) *p = (uint8_t)((*p * keep + 128) >> 8);
  }
}

namespace {

// round((1 − amount)·256) of flapShadeUpper (0.25) and flapShadeLower (0.15).
constexpr uint32_t kKeepUpper = 192;
constexpr uint32_t kKeepLower = 218;

// Copies rows [r0, r1) of a w-wide leaf to the canvas at (x, y + r0).
void blitRows(Canvas& c, int x, int y, int w, const uint8_t* leaf, int r0, int r1) {
  for (int r = r0; r < r1; r++) memcpy(c.at(x, y + r), leaf + (size_t)r * w * 3, (size_t)w * 3);
}

// Scales rows [s0, s0+sh) of a w-wide leaf vertically to dh rows at canvas
// rows y0… (xdraw.ApproxBiLinear with an unchanged width: sx = dx exactly).
// Weights are 8-bit fixed point; values take Go's 16-bit path (c·0x101,
// truncated, then >> 8).
void scaleRows(Canvas& c, int x, int y0, int w, const uint8_t* leaf, int s0, int sh, int dh) {
  const int den = 2 * dh;
  for (int dy = 0; dy < dh; dy++) {
    // 2·dh·sy with sy = (dy + 0.5)·sh/dh − 0.5.
    int n2 = (2 * dy + 1) * sh - dh;
    int sy0, sy1;
    uint32_t f;
    if (n2 < 0) {
      sy0 = sy1 = 0;
      f = 0;
    } else {
      sy0 = n2 / den;
      f = (uint32_t)(((long long)(n2 % den) << 8) / den);
      sy1 = sy0 + 1;
      if (sy1 > sh - 1) {
        sy0 = sy1 = sh - 1;
        f = 0;
      }
    }
    const uint8_t* a = leaf + (size_t)(s0 + sy0) * w * 3;
    const uint8_t* b = leaf + (size_t)(s0 + sy1) * w * 3;
    uint8_t* d = c.at(x, y0 + dy);
    const uint32_t f0 = 256 - f;
    for (int n = w * 3; n > 0; n--) {
      uint32_t v16 = (257u * (*a++ * f0 + *b++ * f)) >> 8;
      *d++ = (uint8_t)(v16 >> 8);
    }
  }
}

}  // namespace

void composeFlip(Canvas& c, int x, int y, int w, int h, const uint8_t* from, const uint8_t* to, int splitY,
                 int phase, int phases) {
  blitRows(c, x, y, w, to, 0, splitY);
  blitRows(c, x, y, w, from, splitY, h);
  bool upper;
  int num, den;
  flipPhase(phase, phases, &upper, &num, &den);
  if (upper) {
    int hh = squashedHeight(splitY, den - num, den);
    scaleRows(c, x, y + splitY - hh, w, from, 0, splitY, hh);
    shade(c, x, y + splitY - hh, x + w, y + splitY, kKeepUpper);
    return;
  }
  int hh = squashedHeight(h - splitY, num, den);
  scaleRows(c, x, y + splitY, w, to, splitY, h - splitY, hh);
  shade(c, x, y + splitY, x + w, y + splitY + hh, kKeepLower);
}

// ---- splitflap layer (splitflap.go) ----

namespace {

using rd::RenderError;

using rd::kMaxBlockDepth;
using rd::kMaxCanvasDimension;

bool withinCanvasRange(int v) { return v >= -kMaxCanvasDimension && v <= kMaxCanvasDimension; }

// Reel: a string (one entry per rune) or a list of strings.
int reelSize(JsonVariantConst reel) {
  if (reel.is<const char*>()) return (int)runeCount(reel.as<const char*>());
  if (reel.is<JsonArrayConst>()) return (int)reel.as<JsonArrayConst>().size();
  return 0;
}

std::string reelEntry(JsonVariantConst reel, int idx) {
  if (reel.is<const char*>()) {
    const char* s = reel.as<const char*>();
    size_t n = strlen(s);
    for (size_t i = 0; i < n;) {
      uint32_t r;
      int wd = decodeRune(s + i, n - i, &r);
      if (idx-- == 0) {
        std::string out;
        appendRune(out, r);
        return out;
      }
      i += wd;
    }
    return std::string();
  }
  JsonVariantConst v = reel[idx];
  return v.is<const char*>() ? v.as<const char*>() : "";
}

// validateFlapChildren: child types a module cannot render, in every branch.
void validateFlapChildren(JsonArrayConst layers, int depth) {
  if (depth > kMaxBlockDepth) throw RenderError("block nesting depth exceeds maximum 16");
  for (JsonVariantConst lv : layers) {
    std::string type = rd::str(lv.as<JsonObjectConst>(), "type");
    if (type == "loop" || type == "copy" || type == "splitflap")
      throw RenderError("layer type \"" + type + "\" is not allowed inside splitflap");
    if (type.empty()) validateFlapChildren(lv["layers"].as<JsonArrayConst>(), depth + 1);
  }
}

std::string rectStr(int x0, int y0, int x1, int y1) {
  return "(" + std::to_string(x0) + "," + std::to_string(y0) + ")-(" + std::to_string(x1) + "," +
         std::to_string(y1) + ")";
}

// Restores the pass's canvas when a leaf is done.
struct CanvasScope {
  Canvas& canvas;
  Canvas saved;
  explicit CanvasScope(Canvas& c) : canvas(c), saved(c) {}
  ~CanvasScope() { canvas = saved; }
};

}  // namespace

void Renderer::Impl::renderLeaf(JsonObjectConst L, const Eval& ev, int x, int y, int w, int h,
                                const std::string& value, bool fresh, uint8_t* out) {
  for (int r = 0; r < h; r++) memcpy(out + (size_t)r * w * 3, canvas.at(x, y + r), (size_t)w * 3);
  Eval child = ev;  // text ids set by the children stay private to the leaf
  std::string var = rd::str(L, "var");
  if (!var.empty()) child.setStrVar(var, value);
  CanvasScope scope(canvas);
  canvas.w = w;
  canvas.h = h;
  canvas.px = out;
  int outer = layerCount;
  if (fresh) layerCount = 0;  // else the still image counts the children
  try {
    renderLayers(L["layers"].as<JsonArrayConst>(), child, false, 0);
  } catch (const RenderError& e) {
    throw RenderError(std::string("renderLeaf: ") + e.what());
  }
  if (fresh) layerCount = outer;
}

void Renderer::Impl::renderSplitflap(JsonObjectConst L, Eval& ev) {
  const int layerIdx = flapLayerSeq++;
  try {
    // planModules
    int x = rd::coord(L, "x", ev), y = rd::coord(L, "y", ev);
    int w = rd::coord(L, "width", ev), h = rd::coord(L, "height", ev);
    if (w <= 0 || h <= 0)
      throw RenderError("width and height must be positive (got " + std::to_string(w) + "x" + std::to_string(h) +
                        ")");
    int cells = rd::plainInt(L["cells"]);
    if (cells == 0) cells = 1;
    else if (cells < 0) throw RenderError("cells must not be negative (got " + std::to_string(cells) + ")");
    else if (cells > kMaxFlapModules)
      throw RenderError("cells " + std::to_string(cells) + " exceeds maximum 64");
    JsonVariantConst reel = L["reel"];
    int reelN = reelSize(reel);
    if (reelN > kMaxReelEntries)
      throw RenderError("reel has " + std::to_string(reelN) + " entries, exceeds maximum 64");
    int ff = rd::plainInt(L["flip_frames"]);
    if (ff != 0 && (ff < 1 || ff > kMaxFlipFrames))
      throw RenderError("flip_frames must be between 1 and 6 (got " + std::to_string(ff) + ")");
    int pitch = rd::plainInt(L["pitch"]);
    if (pitch == 0) pitch = w;
    int splitY = rd::plainInt(L["split_y"]);
    if (splitY == 0) splitY = h / 2;
    if (splitY < 1 || splitY >= h)
      throw RenderError("split_y " + std::to_string(splitY) + " must be between 1 and " + std::to_string(h - 1));
    std::string value = ev.interpolate(rd::resolveCond(L["value"], ev));
    std::string split = rd::str(L, "split");
    if (split.empty()) {
      if (cells > 1) throw RenderError("cells > 1 requires split: chars");
    } else if (split != "chars") {
      throw RenderError("unknown split \"" + split + "\" (expected chars or none)");
    }
    validateFlapChildren(L["layers"].as<JsonArrayConst>(), 0);
    if (!withinCanvasRange(x) || !withinCanvasRange(y) || w > kMaxCanvasDimension || h > kMaxCanvasDimension ||
        !withinCanvasRange(pitch))
      throw RenderError("module 0 lies outside the canvas (0,0)-(240,240)");
    for (int i = 0; i < cells; i++) {
      int mx = x + i * pitch;
      if (mx < 0 || y < 0 || mx + w > canvas.w || y + h > canvas.h)
        throw RenderError("module " + std::to_string(i) + " " + rectStr(mx, y, mx + w, y + h) +
                          " lies outside the canvas (0,0)-(240,240)");
    }

    // registerModules
    if (nFlapRects + cells > kMaxFlapModules) throw RenderError("total splitflap modules exceed maximum 64");
    for (int i = 0; i < cells; i++) {
      FlapRect m = {x + i * pitch, y, x + i * pitch + w, y + h};
      for (int j = 0; j < nFlapRects; j++) {
        const FlapRect& o = flapRects[j];
        if (m.x0 < o.x1 && o.x0 < m.x1 && m.y0 < o.y1 && o.y0 < m.y1)
          throw RenderError("module " + std::to_string(i) + " " + rectStr(m.x0, m.y0, m.x1, m.y1) +
                            " overlaps another splitflap module " + rectStr(o.x0, o.y0, o.x1, o.y1));
      }
      flapRects[nFlapRects++] = m;
    }

    // Targets: the whole value, or one rune per module ("" when missing).
    std::vector<std::string> targets((size_t)cells);
    if (split.empty()) {
      targets[0] = value;
    } else {
      size_t p = 0;
      for (int i = 0; i < cells && p < value.size(); i++) {
        uint32_t r;
        p += decodeRune(value.data() + p, value.size() - p, &r);
        appendRune(targets[(size_t)i], r);
      }
    }

    ZzaBytes stillLeaf{ZzaAlloc<uint8_t>(budget)};
    if (pass == kPassStill) stillLeaf.resize((size_t)w * h * 3);
    for (int i = 0; i < cells; i++) {
      int mx = x + i * pitch;
      try {
        if (pass == kPassFlapFrame) {
          AnimSession& s = *anim;
          if (modCursor >= s.mods.size() || s.mods[modCursor].layerIdx != layerIdx || s.mods[modCursor].cellIdx != i)
            throw RenderError("module first appeared in frame " + std::to_string(frame) + ", not in frame 0");
          const FlapMod& m = s.mods[modCursor++];
          int rest = m.k * m.flipFrames;
          if (frame >= rest) {
            blitRows(canvas, m.x, m.y, m.w, m.leaf(m.seq[m.k]), 0, m.h);
          } else {
            int step = frame / m.flipFrames, phase = frame % m.flipFrames;
            composeFlip(canvas, m.x, m.y, m.w, m.h, m.leaf(m.seq[step]), m.leaf(m.seq[step + 1]), m.splitY, phase,
                        m.flipFrames);
          }
          continue;
        }

        if (pass == kPassStill) {
          renderLeaf(L, ev, mx, y, w, h, targets[(size_t)i], false, stillLeaf.data());
          blitRows(canvas, mx, y, w, stillLeaf.data(), 0, h);
          continue;
        }

        // kPassFlapBegin: schedule (animState.schedule) and every leaf of the run.
        AnimSession& s = *anim;
        s.mods.emplace_back(budget);
        FlapMod& m = s.mods.back();
        m.layerIdx = layerIdx;
        m.cellIdx = i;
        m.x = mx;
        m.y = y;
        m.w = w;
        m.h = h;
        m.splitY = splitY;
        m.flipFrames = ff == 0 ? kDefaultFlipFrames : ff;
        int maxK = s.flipBudget / m.flipFrames;
        if (maxK < 1) maxK = 1;
        int minF = rd::plainInt(L["min_flips"]), maxF = rd::plainInt(L["max_flips"]);
        if (minF == 0) minF = kDefaultMinFlips;
        if (maxF == 0) maxF = maxK;
        minF = std::min(std::max(minF, 1), maxK);
        maxF = std::min(std::max(maxF, 1), maxK);
        minF = std::min(minF, maxF);
        m.k = flipCount(s.seed, layerIdx, i, minF, maxF);
        if (m.k * m.flipFrames > s.end) s.end = m.k * m.flipFrames;

        // Values of the run (flipSequence), deduplicated into leaves.
        const std::string& target = targets[(size_t)i];
        int targetIdx = -1;
        for (int j = 0; j < reelN && targetIdx < 0; j++)
          if (reelEntry(reel, j) == target) targetIdx = j;
        std::vector<std::string> values;
        for (int j = 0; j <= m.k; j++) {
          std::string v = j == m.k     ? target
                          : reelN > 0 ? reelEntry(reel, flipSeqEntry(reelN, targetIdx, m.k, j))
                                      : std::string();
          int li = 0;
          while (li < (int)values.size() && values[(size_t)li] != v) li++;
          if (li == (int)values.size()) values.push_back(v);
          m.seq[j] = (uint8_t)li;
        }
        m.nLeaves = (int)values.size();
        m.leaves.resize((size_t)m.nLeaves * w * h * 3);
        // The still image's leaf counts on the pass's counter (Go's static pass),
        // the other values on their own (animation frames).
        int tl = m.seq[m.k];
        renderLeaf(L, ev, mx, y, w, h, target, false, m.leaves.data() + (size_t)tl * w * h * 3);
        for (int li = 0; li < m.nLeaves; li++)
          if (li != tl) renderLeaf(L, ev, mx, y, w, h, values[(size_t)li], true, m.leaves.data() + (size_t)li * w * h * 3);
        blitRows(canvas, mx, y, w, m.leaf(tl), 0, h);
      } catch (const RenderError& e) {
        throw RenderError("module " + std::to_string(i) + ": " + e.what());
      }
    }
  } catch (const RenderError& e) {
    throw RenderError(std::string("renderSplitflap: ") + e.what());
  }
}

}  // namespace zza
