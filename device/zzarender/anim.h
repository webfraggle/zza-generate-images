// Animation session of zza_anim_begin / zza_anim_frame: split-flap schedule
// and leaves, static prefix, frame mode, dirty rectangles.
#pragma once
#include <cstdint>
#include <memory>

#include "eval.h"
#include "mem.h"
#include "package.h"
#include "splitflap.h"

namespace zza {

// One split-flap module of the animation, planned in zza_anim_begin.
struct FlapMod {
  int layerIdx, cellIdx;  // as flapModule (running splitflap layer number, module index)
  int x, y, w, h, splitY;
  int flipFrames;
  int k;                        // flips; the module rests from frame k·flipFrames on
  int nLeaves;
  uint8_t seq[kMaxAnimFrames];  // leaf index of the value of step j, j = 0…k (seq[k]: target)
  ZzaBytes leaves;              // nLeaves leaves of w*h*3 bytes (RGB888), rendered in zza_anim_begin

  explicit FlapMod(ZzaBudget* b)
      : layerIdx(0), cellIdx(0), x(0), y(0), w(0), h(0), splitY(0), flipFrames(0), k(0), nLeaves(0), seq(),
        leaves(ZzaAlloc<uint8_t>(b)) {}
  const uint8_t* leaf(int j) const { return leaves.data() + (size_t)j * w * h * 3; }
};

struct AnimSession {
  enum Mode { kStill, kFlap, kFrames };

  Mode mode;
  std::unique_ptr<BudgetJsonDoc> doc;  // data; the evaluators point into it
  JsonVariantConst root;
  zza_time now;   // start time; frame mode advances it per frame (split-flap: frozen)
  int frames;     // including the final still (split-flap)
  bool loop;
  int delayCs;    // split-flap: per frame before the last; frame mode: every frame
  int flipBudget; // frames available for flipping (duration·fps, capped)
  int end;        // first frame in which all modules rest
  uint8_t seed[32];
  ZzaBytes scratch;  // frame being rendered (240*240*3)
  ZzaBytes prefix;   // canvas after the static prefix (empty: none)
  int prefixAt;      // top-level layer index after the prefix (0: none)
  int prefixLayerCount;
  std::unique_ptr<Eval> prefixEval;  // evaluator state after the prefix (text ids)
  ZzaVec<FlapMod> mods;

  explicit AnimSession(ZzaBudget* b)
      : mode(kStill), root(), now(), frames(1), loop(false), delayCs(100), flipBudget(0), end(0), seed(),
        scratch(ZzaAlloc<uint8_t>(b)), prefix(ZzaAlloc<uint8_t>(b)), prefixAt(0), prefixLayerCount(0),
        mods(ZzaAlloc<FlapMod>(b)) {}
};

// Changed rectangles between the frame in cur and the previous frame in prev
// (both 240×240 RGB888), and copies the changes into prev. Writes at most
// maxRects rectangles (more changes: one enclosing rectangle) and returns
// their number; 0 = unchanged.
int diffFrames(const uint8_t* cur, uint8_t* prev, zza_rect* rects, int maxRects);

}  // namespace zza
