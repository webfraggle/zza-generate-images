// Split-flap motion and drawing: ports of flipCount, flipSequence, flipPhase,
// squashedHeight, composeFlip and shade of internal/renderer/animation.go.
#pragma once
#include <cstdint>

#include "canvas.h"

namespace zza {

// Limits of internal/renderer (splitflap.go, animation.go).
constexpr int kMaxFlapModules = 64;
constexpr int kMaxReelEntries = 64;
constexpr int kMaxFlipFrames = 6;
constexpr int kDefaultFlipFrames = 2;
constexpr int kDefaultMinFlips = 4;
constexpr int kMaxAnimFrames = 125;  // frames per animation, including the final still

// Seeded number of flips of a module in [minFlips, maxFlips] (minFlips <= maxFlips):
// minFlips + BE64(sha256(seed ‖ BE64(layerIdx) ‖ BE64(cellIdx))[:8]) % (span).
int flipCount(const uint8_t seed[32], int layerIdx, int cellIdx, int minFlips, int maxFlips);

// flipSequence as reel indices: the reel entry shown in step j < k of a run of
// k flips ending on the target (reelN > 0); targetIdx is the first reel index
// equal to the target, or -1. Without a reel every step shows "".
int flipSeqEntry(int reelN, int targetIdx, int k, int j);

// flipPhase: the falling half and its progress p = num/den in (0, 1).
void flipPhase(int phase, int phases, bool* upper, int* num, int* den);

// round(full·num/den), at least 1 and at most full (0 <= num <= den).
int squashedHeight(int full, int num, int den);

// Darkens the colour channels in [x0,x1)×[y0,y1) by keep/256 (rounded).
void shade(Canvas& c, int x0, int y0, int x1, int y1, uint32_t keep);

// composeFlip: draws at (x, y) a w×h module flipping from leaf `from` to leaf
// `to` (RGB888, w*h*3 bytes each) in frame phase of phases; splitY is the
// hinge relative to the module. The squashed half is scaled bilinearly with
// fixed-point weights like xdraw.ApproxBiLinear.
void composeFlip(Canvas& c, int x, int y, int w, int h, const uint8_t* from, const uint8_t* to, int splitY,
                 int phase, int phases);

}  // namespace zza
