// Internal C++ header of the renderer core.
#pragma once
#include <memory>
#include <string>

#include "mem.h"
#include "package.h"
#include "zzarender_c.h"

struct zza_renderer {
  ZzaBudget budget;
  std::string error;  // small; last error text
  std::unique_ptr<zza::Package> pkg;  // loaded package (null before zza_load / after a failed one)

  explicit zza_renderer(size_t mem_budget) : budget(mem_budget) {}
};
