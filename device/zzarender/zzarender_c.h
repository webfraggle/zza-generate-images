// C API of the ZZA on-device renderer core. No exception crosses this boundary.
#pragma once
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct zza_renderer zza_renderer;
typedef struct { int year, month, day, hour, minute, second, weekday; } zza_time; /* weekday 0=So */
typedef struct { int x, y, w, h; } zza_rect;
typedef struct { const char* name; const unsigned char* data; size_t len; } zza_file;

enum { ZZA_OK = 0, ZZA_ERR_FORMAT = 1, ZZA_ERR_TEMPLATE = 2, ZZA_ERR_NOMEM = 3, ZZA_ERR_DATA = 4, ZZA_ERR_STATE = 5 };

zza_renderer* zza_new(size_t mem_budget);
void          zza_delete(zza_renderer*);
const char*   zza_version(void); /* "zzarender/1" */
int           zza_load(zza_renderer*, const zza_file* files, int n);
int           zza_render(zza_renderer*, const char* data, size_t len, const zza_time* now, unsigned char* rgb888);
int           zza_anim_begin(zza_renderer*, const char* data, size_t len, const zza_time* now,
                             int* frames, int* loop);
int           zza_anim_frame(zza_renderer*, int i, unsigned char* rgb888, int* delay_cs,
                             zza_rect* dirty, int max_rects, int* n_rects);
const char*   zza_error(const zza_renderer*);
size_t        zza_mem_used(const zza_renderer*);
size_t        zza_mem_peak(const zza_renderer*);

/* ---- test only (always compiled in; used by the Go test hooks) ---- */
int zza_test_alloc(zza_renderer*, size_t n);                              /* allocate+free n bytes via budget */
int zza_test_sha256(const unsigned char* data, size_t len, unsigned char out[32]);
/* Evaluates expr (interpolation, or condition if isCond: "true"/"false") against dataJson into out
   (NUL-terminated, truncated to outLen). regexJson (may be NULL) is a `_re` table
   {"<pattern>": <program>, ...} for match. Returns the full result length (>= 0), or -ZZA_ERR_*. */
int zza_test_eval(const char* dataJson, const char* regexJson, const zza_time* now, int frame, int frames,
                  int delayCs, const char* expr, int isCond, char* out, size_t outLen);
/* Coordinate expression; varsJson is an object of ints. ZZA_OK, or an error if the expression is
   invalid or the result does not fit an int. */
int zza_test_intexpr(const char* expr, const char* varsJson, int* out);
/* Runs a regex program (JSON [start,ncap,[[op,out,arg,[runes]],...]]) on in[0..inLen) and writes
   capture group `group` (Go matchGroup semantics) to out (NUL-terminated, truncated to outLen).
   Returns the full group length (>= 0), or -ZZA_ERR_FORMAT for an invalid program, -ZZA_ERR_*. */
int zza_test_regex(const char* progJson, const char* in, size_t inLen, int group, char* out, size_t outLen);
/* UTF-8 -> Latin-1 ('?' for runes above U+00FF; strict: fail instead). Returns length or -1. */
int zza_test_latin1(const char* in, size_t len, int strict, unsigned char* out, size_t outLen);

/* FNV-1a 32 of the decoded pixel data (image id "iN") or of the decoded mask area (font id "fN") of the
   loaded package. ZZA_ERR_STATE if no package / no such id. */
int zza_test_hash(zza_renderer*, const char* id, unsigned int* out);

#ifdef __cplusplus
}
#endif
