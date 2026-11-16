// Internal C++ header of the renderer core.
#pragma once
#include <string>

#include "mem.h"
#include "zzarender_c.h"

struct zza_renderer {
  ZzaBudget budget;
  std::string error;  // small; last error text

  explicit zza_renderer(size_t mem_budget) : budget(mem_budget) {}
};
