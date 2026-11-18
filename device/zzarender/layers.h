// Renderer of a loaded package: still images (layer walk with if/elif/else
// chains, block nodes, loops, limits and all layer types; port of
// renderLayers and the layer renderers of internal/renderer) and animations
// (split-flap and frame mode, port of RenderAnimation).
#pragma once
#include <cstddef>
#include <cstdint>
#include <memory>
#include <string>

#include "mem.h"
#include "package.h"
#include "work.h"
#include "zzarender_c.h"

namespace zza {

// Data JSON larger than this is rejected with ZZA_ERR_DATA (ruling R7).
constexpr size_t kMaxDataBytes = 256 * 1024;

class Renderer {
 public:
  // pkg must be loaded and outlive the Renderer.
  Renderer(ZzaBudget* budget, const Package* pkg);
  ~Renderer();
  Renderer(const Renderer&) = delete;
  Renderer& operator=(const Renderer&) = delete;

  // Renders a still image (animation frame 0) into rgb888 (240*240*3 bytes,
  // starts black). Returns ZZA_OK, ZZA_ERR_DATA (invalid or too large data),
  // or ZZA_ERR_TEMPLATE (what Go reports as a render error) with err set.
  // Throws std::bad_alloc when the budget is exhausted (the scaled-image
  // cache is dropped then, so only the package stays allocated).
  int render(const char* data, size_t len, const zza_time& now, uint8_t* rgb888, std::string& err);

  // Starts an animation (replacing a running one): parses data like render,
  // plans the frames and renders the split-flap leaves. *frames: number of
  // frames (split-flap incl. the final still; frame mode meta.animation.frames;
  // otherwise 1), *loop: GIF loop flag as Go. Split-flap seed: SHA-256(manifest
  // name ‖ 0x00 ‖ data) like AnimationSeed.
  // Returns ZZA_OK, ZZA_ERR_DATA or ZZA_ERR_TEMPLATE with err; throws
  // std::bad_alloc (the session and caches are dropped then).
  int animBegin(const char* data, size_t len, const zza_time& now, int* frames, int* loop,
                std::string& err);

  // Renders frame i into rgb888, which must hold frame i-1 (frame 0: anything),
  // see zza_anim_frame. ZZA_ERR_STATE without a session or for i out of range.
  // Throws std::bad_alloc (the session and caches are dropped then).
  int animFrame(int i, uint8_t* rgb888, int* delayCs, zza_rect* dirty, int maxRects, int* nRects,
                std::string& err);

  // Ends the animation and frees its memory.
  void animEnd();

  // Work limit of each render / animBegin / animFrame call (0: kDefaultWorkLimit);
  // exceeding it returns ZZA_ERR_TEMPLATE "Rechenaufwand zu hoch".
  void setWorkLimit(uint64_t ops);
  // Work charged by the last render / animBegin / animFrame call.
  uint64_t workUsed() const;

 private:
  struct Impl;
  std::unique_ptr<Impl> d_;
};

}  // namespace zza
