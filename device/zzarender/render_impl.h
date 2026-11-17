// Internal state of zza::Renderer, shared by the layer walk (layers.cpp),
// splitflap (splitflap.cpp) and the animation driver (anim.cpp).
#pragma once
#include <memory>
#include <stdexcept>
#include <string>

#include "anim.h"
#include "canvas.h"
#include "eval.h"
#include "layers.h"
#include "package.h"

namespace zza {
namespace rd {

// What Go reports as a render error.
struct RenderError : std::runtime_error {
  explicit RenderError(const std::string& m) : std::runtime_error(m) {}
};

// Field accessors of t.json layers (see layers.cpp).
const char* str(JsonObjectConst o, const char* k);
double num(JsonObjectConst o, const char* k);
int plainInt(JsonVariantConst v);
int coord(JsonObjectConst o, const char* k, const Eval& ev);
std::string resolveCond(JsonVariantConst v, const Eval& ev);

// Limits of internal/renderer.
constexpr int kMaxBlockDepth = 16;
constexpr int kMaxCanvasDimension = 16384;

constexpr int kCanvasW = 240;
constexpr int kCanvasH = 240;
constexpr size_t kCanvasBytes = (size_t)kCanvasW * kCanvasH * 3;

}  // namespace rd

struct Renderer::Impl {
  // One cached scaled image; src == nullptr marks a free slot. The slots are
  // fixed (no allocation per entry); pixels are budget memory.
  struct CacheEnt {
    const Img* src;
    int w, h;
    uint32_t used;
    Img img;
    explicit CacheEnt(ZzaBudget* b) : src(nullptr), w(0), h(0), used(0), img(b) {}
  };
  static constexpr size_t kCacheEntries = 4;

  // How a pass draws splitflap modules.
  enum Pass {
    kPassStill,      // still image: each module shows its target
    kPassFlapBegin,  // still image that also plans the animation (schedules, all leaves, prefix)
    kPassFlapFrame,  // animation frame `frame` of the session
  };

  struct FlapRect {
    int x0, y0, x1, y1;
  };

  ZzaBudget* budget;
  const Package* pkg;
  CacheEnt cache[kCacheEntries];
  uint32_t tick;

  // Per pass.
  Canvas canvas;
  int layerCount;
  Pass pass;
  int frame;          // kPassFlapFrame: frame index
  int flapLayerSeq;   // running number of splitflap layers in walk order
  int nFlapRects;     // modules placed so far (overlap / limit check)
  FlapRect flapRects[kMaxFlapModules];
  size_t modCursor;   // kPassFlapFrame: next module of the session
  int snapAt;         // kPassFlapBegin: top-level index at which the prefix is saved (-1: none)

  std::unique_ptr<AnimSession> anim;  // active zza_anim_begin session

  Impl(ZzaBudget* b, const Package* p);

  void clearCache();
  const Img& cachedScale(const Img* src, int w, int h);

  // Parses the data JSON (R7 limits). Returns ZZA_OK or ZZA_ERR_DATA with err.
  int parseData(const char* data, size_t len, std::unique_ptr<BudgetJsonDoc>& doc, JsonVariantConst& root,
                std::string& err);
  // Root evaluator of a render: data, now, regexes; frame mode: anim frame 0.
  Eval rootEval(JsonVariantConst root, const zza_time& now) const;
  // Resets the per-pass state for a pass drawing on px (240×240 RGB888).
  void beginPass(uint8_t* px, Pass p);

  // Layer walk. top marks the template's layer list: there the prefix is saved
  // (kPassFlapBegin) and layers before `start` are skipped.
  void renderLayers(JsonArrayConst layers, Eval& ev, bool inLoop, int depth, bool top = false, int start = 0);
  void renderImage(JsonObjectConst L, const Eval& ev);
  void renderRect(JsonObjectConst L, const Eval& ev);
  void renderText(JsonObjectConst L, Eval& ev);
  void renderCopy(JsonObjectConst L, const Eval& ev);
  void renderLoop(JsonObjectConst L, const Eval& ev, int depth);

  // splitflap.cpp
  void renderSplitflap(JsonObjectConst L, Eval& ev);
  // Renders the leaf of a module showing value into out (w*h*3 bytes): the
  // canvas under the module, the children drawn on it. fresh: own layer
  // counter (animation leaves), else the pass's counter (still image).
  void renderLeaf(JsonObjectConst L, const Eval& ev, int x, int y, int w, int h, const std::string& value,
                  bool fresh, uint8_t* out);

  // anim.cpp
  void snapshotPrefix(const Eval& ev);
  void dropAnim();
};

}  // namespace zza
