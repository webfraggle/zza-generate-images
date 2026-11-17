// Layer walk of a still image: if/elif/else chains, block nodes, loops,
// limits and the image/rect/text/copy layers. Port of renderLayers and the
// layer renderers of internal/renderer.
#pragma once
#include <cstddef>
#include <cstdint>
#include <memory>
#include <string>

#include "mem.h"
#include "package.h"
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

 private:
  struct Impl;
  std::unique_ptr<Impl> d_;
};

}  // namespace zza
