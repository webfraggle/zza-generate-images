#include <cstdlib>
#include <exception>
#include <new>

#include "sha256.h"
#include "zzarender.h"

void* (*zza_raw_malloc)(size_t) = malloc;
void (*zza_raw_free)(void*) = free;

// Runs f at an API boundary: bad_alloc -> NOMEM, std::exception -> TEMPLATE,
// anything else -> TEMPLATE. Never lets an exception escape.
template <class F>
static int guarded(zza_renderer* r, F f) {
  try {
    return f();
  } catch (const std::bad_alloc&) {
    if (r) r->error = "out of memory";
    return ZZA_ERR_NOMEM;
  } catch (const std::exception& e) {
    if (r) {
      try { r->error = e.what(); } catch (...) {}
    }
    return ZZA_ERR_TEMPLATE;
  } catch (...) {
    if (r) {
      try { r->error = "unknown error"; } catch (...) {}
    }
    return ZZA_ERR_TEMPLATE;
  }
}

static int not_implemented(zza_renderer* r) {
  return guarded(r, [&]() {
    if (r) r->error = "not implemented";
    return (int)ZZA_ERR_STATE;
  });
}

extern "C" {

zza_renderer* zza_new(size_t mem_budget) {
  try {
    return new (std::nothrow) zza_renderer(mem_budget);
  } catch (...) {
    return nullptr;
  }
}

void zza_delete(zza_renderer* r) { delete r; }

const char* zza_version(void) { return "zzarender/1"; }

int zza_load(zza_renderer* r, const zza_file*, int) { return not_implemented(r); }

int zza_render(zza_renderer* r, const char*, size_t, const zza_time*, unsigned char*) {
  return not_implemented(r);
}

int zza_anim_begin(zza_renderer* r, const char*, size_t, const zza_time*, int*, int*) {
  return not_implemented(r);
}

int zza_anim_frame(zza_renderer* r, int, unsigned char*, int*, zza_rect*, int, int*) {
  return not_implemented(r);
}

const char* zza_error(const zza_renderer* r) { return r ? r->error.c_str() : ""; }
size_t zza_mem_used(const zza_renderer* r) { return r ? r->budget.used : 0; }
size_t zza_mem_peak(const zza_renderer* r) { return r ? r->budget.peak : 0; }

int zza_test_alloc(zza_renderer* r, size_t n) {
  if (!r) return ZZA_ERR_STATE;
  return guarded(r, [&]() {
    ZzaBytes b{ZzaAlloc<uint8_t>(&r->budget)};
    b.resize(n);
    return (int)ZZA_OK;
  });
}

int zza_test_sha256(const unsigned char* data, size_t len, unsigned char out[32]) {
  return guarded(nullptr, [&]() {
    ZzaSha256 s;
    s.update(data, len);
    s.final(out);
    return (int)ZZA_OK;
  });
}

}  // extern "C"
