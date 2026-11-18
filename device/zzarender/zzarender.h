// Internal C++ header of the renderer core.
#pragma once
#include <cstdint>
#include <memory>
#include <string>

#include "layers.h"
#include "mem.h"
#include "package.h"
#include "zzarender_c.h"

struct zza_renderer {
  ZzaBudget budget;
  uint64_t workLimit;  // per call (zza_set_work_limit); 0: the default
  std::string error;  // small; last error text
  std::unique_ptr<zza::Package> pkg;  // loaded package (null before zza_load / after a failed one)
  std::unique_ptr<zza::Renderer> rend;  // renders pkg; declared after it, so destroyed first

  explicit zza_renderer(size_t mem_budget) : budget(mem_budget), workLimit(0) {}
};
