// Animation driver: port of RenderAnimation / renderFrameMode of
// internal/renderer/animation.go for frame-by-frame rendering on the device,
// plus the static prefix and the dirty rectangles.
#include "anim.h"

#include <cmath>
#include <cstring>

#include "render_impl.h"
#include "sha256.h"

namespace zza {

namespace {

using rd::RenderError;

constexpr int kDefaultFPS = 20, kMinFPS = 5, kMaxFPS = 25;
constexpr double kDefaultDuration = 2.5, kMinDuration = 0.5, kMaxDuration = 5.0;
constexpr int kMinAnimFrames = 2;
constexpr int kFinalDelayCs = 100;  // gifFinalDelay: the still stays 1 s
constexpr int kMinFrameDelayCs = 2, kMaxFrameDelayCs = 1000;
constexpr int kMaxAnimSeconds = 120;

// Resolved meta.animation (AnimationMeta.resolve and validateAnimation).
struct AnimMeta {
  int fps;
  double duration;
  int frames;   // frame mode; 0 = none
  int delayCs;  // frame mode
  bool loop;
};

// Returns "" or what Go reports for an invalid meta.animation.
std::string resolveMeta(JsonObjectConst an, AnimMeta* m) {
  m->fps = rd::plainInt(an["fps"]);
  m->duration = rd::num(an, "duration");
  m->frames = rd::plainInt(an["frames"]);
  JsonVariantConst lv = an["loop"];
  m->loop = lv.is<bool>() && lv.as<bool>();
  if (m->fps == 0) m->fps = kDefaultFPS;
  else if (m->fps < kMinFPS || m->fps > kMaxFPS)
    return "animation.fps must be between 5 and 25 (got " + std::to_string(m->fps) + ")";
  if (m->duration == 0) m->duration = kDefaultDuration;
  else if (!(m->duration >= kMinDuration && m->duration <= kMaxDuration))
    return "animation.duration must be between 0.5 and 5 seconds";
  m->delayCs = 0;
  if (m->frames == 0) {
    if (rd::num(an, "frame_delay") != 0) return "animation.frame_delay requires animation.frames";
    return "";
  }
  if (m->frames < kMinAnimFrames || m->frames > kMaxAnimFrames)
    return "animation.frames must be between 2 and 125 (got " + std::to_string(m->frames) + ")";
  if (rd::plainInt(an["fps"]) != 0 || rd::num(an, "duration") != 0)
    return "animation.frames cannot be combined with animation.fps or animation.duration";
  double d = rd::num(an, "frame_delay");
  if (d == 0) d = 1;
  double cs = d * 100;
  if (!(cs >= kMinFrameDelayCs - 1e-9 && cs <= kMaxFrameDelayCs + 1e-9) || std::fabs(cs - std::round(cs)) >= 1e-9)
    return "animation.frame_delay must be between 0.02 and 10 seconds in steps of 0.01";
  if (m->frames * d > kMaxAnimSeconds) return "animation.frames × animation.frame_delay must not exceed 120 seconds";
  m->delayCs = (int)std::round(cs);
  return "";
}

// hasSplitflap: a splitflap layer at this level or in block nodes below.
bool hasSplitflap(JsonArrayConst layers, int depth) {
  if (depth > 64) return false;  // deeper nesting fails in the walk anyway
  for (JsonVariantConst lv : layers) {
    JsonObjectConst L = lv.as<JsonObjectConst>();
    std::string type = rd::str(L, "type");
    if (type == "splitflap") return true;
    if (type.empty() && hasSplitflap(L["layers"].as<JsonArrayConst>(), depth + 1)) return true;
  }
  return false;
}

bool isElifOrElse(JsonObjectConst L) {
  JsonVariantConst e = L["else"];
  return *rd::str(L, "elif") || (e.is<bool>() && e.as<bool>());
}

}  // namespace

void Renderer::Impl::snapshotPrefix(const Eval& ev) {
  AnimSession& s = *anim;
  memcpy(s.prefix.data(), canvas.px, rd::kCanvasBytes);
  s.prefixEval.reset(new Eval(ev));
  s.prefixLayerCount = layerCount;
}

void Renderer::Impl::dropAnim() {
  anim.reset();
  clearCache();
}

int Renderer::animBegin(const char* data, size_t len, const zza_time& now, int* frames, int* loop,
                        std::string& err) {
  Impl& d = *d_;
  d.anim.reset();  // a new animation replaces the running one
  d.resetWork();
  try {
    std::unique_ptr<AnimSession> s(new AnimSession(d.budget));
    int rc = d.parseData(data, len, s->doc, s->root, err);
    if (rc != ZZA_OK) return rc;
    s->now = now;

    JsonObjectConst t = d.pkg->tmpl();
    AnimMeta meta = {};
    std::string merr = resolveMeta(t["meta"]["animation"], &meta);
    if (!merr.empty()) {
      err = merr;
      return ZZA_ERR_TEMPLATE;
    }
    JsonArrayConst layers = t["layers"];
    s->scratch.resize(rd::kCanvasBytes);
    s->loop = false;
    s->frames = 1;
    s->delayCs = kFinalDelayCs;

    if (meta.frames > 0 && hasSplitflap(layers, 0)) {
      err = "animation.frames cannot be used in a template with splitflap layers";
      return ZZA_ERR_TEMPLATE;
    }
    if (meta.frames > 0) {
      s->mode = AnimSession::kFrames;
      s->frames = meta.frames;
      s->loop = meta.loop;
      s->delayCs = meta.delayCs;
      d.anim = std::move(s);
    } else if (!hasSplitflap(layers, 0)) {
      s->mode = AnimSession::kStill;
      d.anim = std::move(s);
    } else {
      s->mode = AnimSession::kFlap;
      int budget = (int)std::floor(meta.duration * meta.fps + 1e-9);
      s->flipBudget = budget < kMaxAnimFrames - 1 ? budget : kMaxAnimFrames - 1;
      ZzaSha256 h;
      const std::string& name = d.pkg->name();
      h.update(reinterpret_cast<const uint8_t*>(name.data()), name.size());
      uint8_t zero = 0;
      h.update(&zero, 1);
      if (len > 0) h.update(reinterpret_cast<const uint8_t*>(data), len);
      h.final(s->seed);
      s->mods.reserve(kMaxFlapModules);

      // Static prefix: top-level layers before the first one containing a
      // splitflap never change (now frozen, anim.* constant) — unless that
      // layer continues an if chain.
      int at = 0;
      for (JsonVariantConst lv : layers) {
        JsonObjectConst L = lv.as<JsonObjectConst>();
        std::string type = rd::str(L, "type");
        if (type == "splitflap" || (type.empty() && hasSplitflap(L["layers"].as<JsonArrayConst>(), 1))) {
          if (isElifOrElse(L)) at = 0;
          break;
        }
        at++;
      }
      s->prefixAt = at;
      if (at > 0) s->prefix.resize(rd::kCanvasBytes);

      // The still image (Go's static pass) plans every module and renders its leaves.
      d.anim = std::move(s);
      AnimSession& a = *d.anim;
      Eval ev = d.rootEval(a.root, now);
      memset(a.scratch.data(), 0, rd::kCanvasBytes);
      d.beginPass(a.scratch.data(), Impl::kPassFlapBegin);
      d.snapAt = at > 0 ? at : -1;
      try {
        d.renderLayers(layers, ev, false, 0, true);
      } catch (const std::runtime_error& e) {  // RenderError or WorkLimitError
        d.snapAt = -1;
        d.anim.reset();
        err = e.what();
        return ZZA_ERR_TEMPLATE;
      }
      d.snapAt = -1;
      if (at > 0 && !a.prefixEval) {  // the walk never reached the layer (cannot happen)
        a.prefixAt = 0;
        ZzaBytes(ZzaAlloc<uint8_t>(d.budget)).swap(a.prefix);
      }
      if (a.end > 0) {
        a.frames = a.end + 1;
        a.delayCs = (100 + meta.fps / 2) / meta.fps;
      }
      a.loop = meta.loop;
    }
    *frames = d.anim->frames;
    *loop = d.anim->loop ? 1 : 0;
    return ZZA_OK;
  } catch (const std::bad_alloc&) {
    d.snapAt = -1;
    d.dropAnim();
    throw;
  }
}

void Renderer::animEnd() { d_->anim.reset(); }

int Renderer::animFrame(int i, const zza_time* now, uint8_t* rgb888, int* delayCs, zza_rect* dirty, int maxRects, int* nRects,
                        std::string& err) {
  Impl& d = *d_;
  if (!d.anim) {
    err = "no animation started";
    return ZZA_ERR_STATE;
  }
  AnimSession& a = *d.anim;
  d.resetWork();
  if (i < 0 || i >= a.frames) {
    err = "frame " + std::to_string(i) + " out of range (" + std::to_string(a.frames) + " frames)";
    return ZZA_ERR_STATE;
  }
  try {
    JsonArrayConst layers = d.pkg->tmpl()["layers"];
    uint8_t* px = a.scratch.data();
    try {
      if (a.mode == AnimSession::kFlap) {
        d.beginPass(px, Impl::kPassFlapFrame);
        d.frame = i == a.frames - 1 ? a.end : i;  // the last frame: every module at rest = the still
        if (a.prefixAt > 0) {
          memcpy(px, a.prefix.data(), rd::kCanvasBytes);
          Eval ev = *a.prefixEval;
          d.layerCount = a.prefixLayerCount;
          d.renderLayers(layers, ev, false, 0, true, a.prefixAt);
        } else {
          memset(px, 0, rd::kCanvasBytes);
          Eval ev = d.rootEval(a.root, a.now);
          d.renderLayers(layers, ev, false, 0, true);
        }
      } else {
        memset(px, 0, rd::kCanvasBytes);
        d.beginPass(px, Impl::kPassStill);
        zza_time t = a.now;
        if (now) t = *now;
        else if (a.mode == AnimSession::kFrames) t = timeAddSeconds(a.now, (int64_t)i * a.delayCs / 100);
        Eval ev = d.rootEval(a.root, t);
        if (a.mode == AnimSession::kFrames) ev.setAnim(i, a.frames, a.delayCs);
        d.renderLayers(layers, ev, false, 0, true);
      }
    } catch (const std::runtime_error& e) {  // RenderError or WorkLimitError
      err = "frame " + std::to_string(i) + ": " + e.what();
      return ZZA_ERR_TEMPLATE;
    }
  } catch (const std::bad_alloc&) {
    d.dropAnim();
    throw;
  }

  if (delayCs) *delayCs = (a.mode == AnimSession::kFrames || i < a.frames - 1) ? a.delayCs : kFinalDelayCs;
  int n;
  if (i == 0) {  // the caller's buffer is undefined before frame 0
    memcpy(rgb888, a.scratch.data(), rd::kCanvasBytes);
    n = 0;
    if (dirty && maxRects > 0) {
      dirty[0].x = 0;
      dirty[0].y = 0;
      dirty[0].w = rd::kCanvasW;
      dirty[0].h = rd::kCanvasH;
      n = 1;
    }
  } else {
    n = diffFrames(a.scratch.data(), rgb888, dirty, dirty ? maxRects : 0);
  }
  if (nRects) *nRects = n;
  return ZZA_OK;
}

// ---- dirty rectangles ----

namespace {

// Column runs closer than this are reported as one rectangle.
constexpr int kMergeGap = 16;

struct RectList {
  zza_rect* out;
  int max, n;
  bool overflow;
  int x0, y0, x1, y1;  // union of everything added
  void add(int ax0, int ay0, int ax1, int ay1) {
    if (n == 0 && !overflow) {
      x0 = ax0, y0 = ay0, x1 = ax1, y1 = ay1;
    } else {
      x0 = ax0 < x0 ? ax0 : x0;
      y0 = ay0 < y0 ? ay0 : y0;
      x1 = ax1 > x1 ? ax1 : x1;
      y1 = ay1 > y1 ? ay1 : y1;
    }
    if (n < max) {
      out[n].x = ax0;
      out[n].y = ay0;
      out[n].w = ax1 - ax0;
      out[n].h = ay1 - ay0;
      n++;
    } else {
      overflow = true;
    }
  }
};

// Emits the column runs of a band of changed rows [y0, y1); runs less than
// kMergeGap apart are joined.
void flushBand(RectList& rl, const uint8_t* cols, int y0, int y1) {
  int x = 0;
  for (;;) {
    while (x < rd::kCanvasW && !cols[x]) x++;
    if (x >= rd::kCanvasW) return;
    int start = x, end = ++x;
    while (x < rd::kCanvasW && x - end < kMergeGap) {
      if (cols[x]) end = x + 1;
      x++;
    }
    rl.add(start, y0, end, y1);
    x = end;
  }
}

}  // namespace

int diffFrames(const uint8_t* cur, uint8_t* prev, zza_rect* rects, int maxRects) {
  const size_t row = (size_t)rd::kCanvasW * 3;
  RectList rl = {rects, maxRects > 0 ? maxRects : 0, 0, false, 0, 0, 0, 0};
  uint8_t cols[rd::kCanvasW];  // columns changed in the current band
  int band = -1;               // first row of the current band of changed rows
  for (int y = 0; y <= rd::kCanvasH; y++) {
    bool changed = y < rd::kCanvasH && memcmp(cur + y * row, prev + y * row, row) != 0;
    if (!changed) {
      if (band >= 0) {
        flushBand(rl, cols, band, y);
        band = -1;
      }
      continue;
    }
    if (band < 0) {
      band = y;
      memset(cols, 0, sizeof cols);
    }
    const uint8_t* c = cur + y * row;
    uint8_t* p = prev + y * row;
    for (int x = 0; x < rd::kCanvasW; x++, c += 3, p += 3) {
      if (c[0] != p[0] || c[1] != p[1] || c[2] != p[2]) {
        cols[x] = 1;
        p[0] = c[0];
        p[1] = c[1];
        p[2] = c[2];
      }
    }
  }
  if (rl.overflow && rl.max > 0) {
    rects[0].x = rl.x0;
    rects[0].y = rl.y0;
    rects[0].w = rl.x1 - rl.x0;
    rects[0].h = rl.y1 - rl.y0;
    return 1;
  }
  return rl.n;
}

}  // namespace zza
