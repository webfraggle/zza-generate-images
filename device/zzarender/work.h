// Bounds of one render call: the work meter (zza_set_work_limit) and the
// errors of the render path.
#pragma once
#include <cstdint>
#include <stdexcept>
#include <string>

#include "zzarender_c.h"

namespace zza {

// What Go reports as a render error (ZZA_ERR_TEMPLATE).
struct RenderError : std::runtime_error {
  explicit RenderError(const std::string& m) : std::runtime_error(m) {}
};

// A call exceeded its work limit (ZZA_ERR_TEMPLATE). Not a RenderError, so
// the layer walk passes it on unchanged.
struct WorkLimitError : std::runtime_error {
  WorkLimitError() : std::runtime_error("Rechenaufwand zu hoch") {}
};

// Default work limit per zza_render / zza_anim_begin / zza_anim_frame call,
// see spec §13 for the measured legitimate maximum.
constexpr uint64_t kDefaultWorkLimit = ZZA_DEFAULT_WORK_LIMIT;

// Counts the dominant operations of one call: pixels written or blended
// (canvas, leaves, scaling taps), glyphs laid out, regex VM thread steps and
// evaluator bytes. Operations are charged before they run, so exceeding the
// limit stops the call before the expensive part.
struct Work {
  uint64_t used;
  uint64_t limit;

  Work() : used(0), limit(kDefaultWorkLimit) {}
  void charge(uint64_t n) {
    if (n > limit - used) {
      used = limit;
      throw WorkLimitError();
    }
    used += n;
  }
};

// Charges w if there is a meter (test hooks run without one).
inline void charge(Work* w, uint64_t n) {
  if (w) w->charge(n);
}

}  // namespace zza
