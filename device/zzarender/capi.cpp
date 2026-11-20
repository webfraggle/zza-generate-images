#include <cstdlib>
#include <cstring>
#include <exception>
#include <new>

#include "eval.h"
#include "regex.h"
#include "sha256.h"
#include "utf8.h"
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

int zza_load(zza_renderer* r, const zza_file* files, int n) {
  if (!r) return ZZA_ERR_STATE;
  return guarded(r, [&]() {
    r->rend.reset();
    r->pkg.reset();  // a failed or repeated load never keeps the old package
    r->error.clear();
    std::unique_ptr<zza::Package> p(new zza::Package(&r->budget));
    std::string err;
    int rc = p->load(files, n, err);
    if (rc != ZZA_OK) {
      r->error = err;
      return rc;
    }
    r->pkg = std::move(p);
    r->rend.reset(new zza::Renderer(&r->budget, r->pkg.get()));
    r->rend->setWorkLimit(r->workLimit);
    return (int)ZZA_OK;
  });
}

int zza_render(zza_renderer* r, const char* data, size_t len, const zza_time* now, unsigned char* rgb888) {
  if (!r) return ZZA_ERR_STATE;
  return guarded(r, [&]() {
    r->error.clear();
    if (!r->pkg || !r->rend) {
      r->error = "no package loaded";
      return (int)ZZA_ERR_STATE;
    }
    if (!now || !rgb888 || (!data && len > 0)) {
      r->error = "invalid arguments";
      return (int)ZZA_ERR_STATE;
    }
    return r->rend->render(data, len, *now, rgb888, r->error);
  });
}

int zza_clock(const zza_renderer* r) { return r && r->pkg ? r->pkg->clock() : ZZA_CLOCK_NONE; }

int zza_render_diff(zza_renderer* r, const char* data, size_t len, const zza_time* now, unsigned char* rgb888,
                    zza_rect* dirty, int max_rects, int* n_rects) {
  if (!r) return ZZA_ERR_STATE;
  return guarded(r, [&]() {
    r->error.clear();
    if (!r->pkg || !r->rend) {
      r->error = "no package loaded";
      return (int)ZZA_ERR_STATE;
    }
    if (!now || !rgb888 || (!data && len > 0)) {
      r->error = "invalid arguments";
      return (int)ZZA_ERR_STATE;
    }
    return r->rend->renderDiff(data, len, *now, rgb888, dirty, max_rects, n_rects, r->error);
  });
}

int zza_anim_begin(zza_renderer* r, const char* data, size_t len, const zza_time* now, int* frames, int* loop) {
  if (!r) return ZZA_ERR_STATE;
  return guarded(r, [&]() {
    r->error.clear();
    if (!r->pkg || !r->rend) {
      r->error = "no package loaded";
      return (int)ZZA_ERR_STATE;
    }
    if (!now || !frames || !loop || (!data && len > 0)) {
      r->error = "invalid arguments";
      return (int)ZZA_ERR_STATE;
    }
    return r->rend->animBegin(data, len, *now, frames, loop, r->error);
  });
}

int zza_anim_frame(zza_renderer* r, int i, const zza_time* now, unsigned char* rgb888, int* delay_cs, zza_rect* dirty, int max_rects,
                   int* n_rects) {
  if (!r) return ZZA_ERR_STATE;
  return guarded(r, [&]() {
    r->error.clear();
    if (!r->pkg || !r->rend) {
      r->error = "no package loaded";
      return (int)ZZA_ERR_STATE;
    }
    if (!rgb888) {
      r->error = "invalid arguments";
      return (int)ZZA_ERR_STATE;
    }
    return r->rend->animFrame(i, now, rgb888, delay_cs, dirty, max_rects, n_rects, r->error);
  });
}

void zza_anim_end(zza_renderer* r) {
  if (r && r->rend) r->rend->animEnd();
}

void zza_set_work_limit(zza_renderer* r, uint64_t ops) {
  if (!r) return;
  r->workLimit = ops;
  if (r->rend) r->rend->setWorkLimit(ops);
}

uint64_t zza_work_used(const zza_renderer* r) { return r && r->rend ? r->rend->workUsed() : 0; }

const char* zza_error(const zza_renderer* r) { return r ? r->error.c_str() : ""; }
size_t zza_mem_used(const zza_renderer* r) { return r ? r->budget.used : 0; }
size_t zza_mem_peak(const zza_renderer* r) { return r ? r->budget.peak : 0; }

#ifdef ZZA_TEST_HOOKS

int zza_test_alloc(zza_renderer* r, size_t n) {
  if (!r) return ZZA_ERR_STATE;
  return guarded(r, [&]() {
    ZzaBytes b{ZzaAlloc<uint8_t>(&r->budget)};
    b.resize(n);
    return (int)ZZA_OK;
  });
}

void zza_test_set_limit(zza_renderer* r, size_t limit) {
  if (r) r->budget.limit = limit;
}

int zza_test_hash(zza_renderer* r, const char* id, unsigned int* out) {
  if (!r || !r->pkg || !id || !out) return ZZA_ERR_STATE;
  const uint8_t* p = nullptr;
  size_t n = 0;
  if (const zza::Img* im = r->pkg->image(id)) {
    p = im->px.data();
    n = im->px.size();
  } else if (const zza::Font* f = r->pkg->font(id)) {
    p = f->masks.data();
    n = f->masks.size();
  } else {
    return ZZA_ERR_STATE;
  }
  uint32_t h = 2166136261u;
  for (size_t i = 0; i < n; i++) h = (h ^ p[i]) * 16777619u;
  *out = h;
  return ZZA_OK;
}

int zza_test_sha256(const unsigned char* data, size_t len, unsigned char out[32]) {
  return guarded(nullptr, [&]() {
    ZzaSha256 s;
    s.update(data, len);
    s.final(out);
    return (int)ZZA_OK;
  });
}

// Budget for the test hooks' scratch memory (regex programs and VM).
static const size_t kTestBudget = 64u << 20;

// Copies res into out (NUL-terminated, truncated to outLen).
static void copyOut(const std::string& res, char* out, size_t outLen) {
  if (out && outLen > 0) {
    size_t c = res.size() < outLen - 1 ? res.size() : outLen - 1;
    memcpy(out, res.data(), c);
    out[c] = 0;
  }
}

int zza_test_eval(const char* dataJson, const char* regexJson, const zza_time* now, int frame, int frames,
                  int delayCs, const char* expr, int isCond, char* out, size_t outLen) {
  if (!dataJson || !now || !expr) return -ZZA_ERR_STATE;
  int len = 0;
  int rc = guarded(nullptr, [&]() {
    size_t n = strlen(dataJson);
    DynamicJsonDocument doc(n * 12 + 1024);
    if (deserializeJson(doc, dataJson, n)) return (int)ZZA_ERR_DATA;
    ZzaBudget budget(kTestBudget);
    zza::RegexTable regex(&budget);
    size_t rn = regexJson ? strlen(regexJson) : 0;
    DynamicJsonDocument reDoc(rn * 32 + 256);
    if (regexJson) {
      if (deserializeJson(reDoc, regexJson, rn)) return (int)ZZA_ERR_DATA;
      if (!regex.load(reDoc.as<JsonObjectConst>())) return (int)ZZA_ERR_FORMAT;
    }
    zza::Eval ev(doc.as<JsonVariantConst>(), *now);
    ev.setAnim(frame, frames, delayCs);
    ev.setRegexSource(&regex);
    std::string res;
    if (isCond) res = ev.cond(expr) ? "true" : "false";
    else res = ev.interpolate(expr);
    len = (int)res.size();
    copyOut(res, out, outLen);
    return (int)ZZA_OK;
  });
  return rc == ZZA_OK ? len : -rc;
}

int zza_test_intexpr(const char* expr, const char* varsJson, int* out) {
  if (!expr || !varsJson || !out) return ZZA_ERR_STATE;
  return guarded(nullptr, [&]() {
    DynamicJsonDocument doc(strlen(varsJson) * 12 + 256);
    if (deserializeJson(doc, varsJson)) return (int)ZZA_ERR_DATA;
    zza::IntVars vars;
    for (JsonPair p : doc.as<JsonObject>()) vars.push_back(zza::IntVar{p.key().c_str(), p.value().as<int>()});
    bool ok = false;
    int v = zza::evalIntExpr(expr, vars, &ok);
    if (!ok) return (int)ZZA_ERR_TEMPLATE;
    *out = v;
    return (int)ZZA_OK;
  });
}

int zza_test_regex(const char* progJson, const char* in, size_t inLen, int group, char* out, size_t outLen) {
  if (!progJson || (!in && inLen > 0)) return -ZZA_ERR_STATE;
  int len = 0;
  int rc = guarded(nullptr, [&]() {
    size_t n = strlen(progJson);
    DynamicJsonDocument doc(n * 32 + 256);  // dense arrays: up to one slot per 2 chars
    if (deserializeJson(doc, progJson, n)) return (int)ZZA_ERR_FORMAT;
    ZzaBudget budget(kTestBudget);
    zza::RegexProg prog(&budget);
    if (!prog.load(doc.as<JsonArrayConst>())) return (int)ZZA_ERR_FORMAT;
    std::string res = zza::matchGroup(prog, std::string_view(in ? in : "", inLen), group);
    len = (int)res.size();
    copyOut(res, out, outLen);
    return (int)ZZA_OK;
  });
  return rc == ZZA_OK ? len : -rc;
}

int zza_test_latin1(const char* in, size_t len, int strict, unsigned char* out, size_t outLen) {
  if ((!in && len > 0) || (!out && outLen > 0)) return -1;
  int res = -1;
  guarded(nullptr, [&]() {
    std::string o;
    if (zza::utf8ToLatin1(std::string_view(in, len), o, strict != 0)) {
      res = (int)o.size();
      if (!o.empty() && o.size() <= outLen) memcpy(out, o.data(), o.size());
    }
    return (int)ZZA_OK;
  });
  return res;
}

#endif  // ZZA_TEST_HOOKS

}  // extern "C"
