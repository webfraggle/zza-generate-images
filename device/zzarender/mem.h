// Budget allocator: all significant core memory goes through ZzaBudget so the
// firmware can cap PSRAM use. Over budget or raw malloc NULL -> std::bad_alloc.
#pragma once
// The core reports budget exhaustion and render errors with exceptions that
// the C API turns into status codes; without exceptions a failed allocation
// would abort the firmware.
#if !defined(__cpp_exceptions)
#error "zzarender requires C++ exceptions (-fexceptions)"
#endif
#include <cstddef>
#include <cstdint>
#include <cstdlib>
#include <new>
#include <vector>

// Platform hook for the raw allocation (default malloc/free; firmware sets PSRAM).
extern void* (*zza_raw_malloc)(size_t);
extern void (*zza_raw_free)(void*);

struct ZzaBudget {
  size_t limit;
  size_t used;
  size_t peak;

  explicit ZzaBudget(size_t lim) : limit(lim), used(0), peak(0) {}

  void* alloc(size_t n) {
    if (n > limit || used > limit - n) throw std::bad_alloc();
    void* p = zza_raw_malloc(n ? n : 1);
    if (!p) throw std::bad_alloc();
    used += n;
    if (used > peak) peak = used;
    return p;
  }

  void free(void* p, size_t n) {
    if (!p) return;
    zza_raw_free(p);
    used = (n <= used) ? used - n : 0;
  }
};

template <class T>
struct ZzaAlloc {
  using value_type = T;
  ZzaBudget* budget;

  explicit ZzaAlloc(ZzaBudget* b) : budget(b) {}
  template <class U>
  ZzaAlloc(const ZzaAlloc<U>& o) : budget(o.budget) {}

  T* allocate(size_t n) {
    if (n > SIZE_MAX / sizeof(T)) throw std::bad_alloc();
    return static_cast<T*>(budget->alloc(n * sizeof(T)));
  }
  void deallocate(T* p, size_t n) { budget->free(p, n * sizeof(T)); }

  template <class U>
  bool operator==(const ZzaAlloc<U>& o) const { return budget == o.budget; }
  template <class U>
  bool operator!=(const ZzaAlloc<U>& o) const { return budget != o.budget; }
};

using ZzaBytes = std::vector<uint8_t, ZzaAlloc<uint8_t>>;
template <class T>
using ZzaVec = std::vector<T, ZzaAlloc<T>>;
