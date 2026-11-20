// C API of the ZZA on-device renderer core. No exception crosses this boundary.
#pragma once
#include <stddef.h>
#include <stdint.h>

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
/* Starts an animation of the loaded package with data (as zza_render; a running animation ends).
   Split-flap: seed SHA-256(manifest name || 0x00 || data[0..len)), schedule like RenderAnimation,
   *frames = frames incl. the final still (= zza_render). Frame mode (meta.animation.frames):
   *frames = animation.frames. Otherwise *frames = 1. *loop: 1 = repeat (GIF loop), 0 = play once.
   now is the start time: frame mode advances it per frame (see zza_anim_frame), split-flap keeps it
   frozen. Leaves, prefix and frame buffer are budget memory; ZZA_ERR_NOMEM
   (here or in zza_anim_frame) ends the animation and frees all of it. */
int           zza_anim_begin(zza_renderer*, const char* data, size_t len, const zza_time* now,
                             int* frames, int* loop);
/* Renders frame i (0 <= i < frames, any order) into rgb888, which must hold frame i-1 (for i = 0:
   anything). *delay_cs: duration of the frame (split-flap round(100/fps), last frame 100; frame mode
   frame_delay). dirty receives up to max_rects rectangles that changed against frame i-1 (frame 0:
   the whole canvas; more changes than max_rects: one enclosing rectangle), *n_rects their number
   (0 = unchanged, or nothing reported for max_rects < 1 / dirty NULL). delay_cs, dirty and n_rects
   may be NULL. ZZA_ERR_STATE without an animation or for i out of range.
   now: NULL = frame mode renders frame i at the start time of zza_anim_begin plus
   floor(i * delay_cs / 100) seconds (carried into the date); split-flap and a still (frames == 1)
   use the start time. now != NULL is the time of this frame in frame mode and for a still; split-flap
   ignores it. */
int           zza_anim_frame(zza_renderer*, int i, const zza_time* now, unsigned char* rgb888, int* delay_cs,
                             zza_rect* dirty, int max_rects, int* n_rects);
/* Ends the animation and frees its memory (leaves, prefix, frame buffer, data). No-op without one. */
void          zza_anim_end(zza_renderer*);
#define ZZA_DEFAULT_WORK_LIMIT 100000000ULL /* operations; see spec §13 */
/* Work limit per zza_render / zza_anim_begin / zza_anim_frame call, in operations (pixels
   written or blended, scaling taps, glyphs laid out, regex VM thread steps, evaluator
   bytes); 0 restores the default (spec §13). A call that would exceed it returns
   ZZA_ERR_TEMPLATE "Rechenaufwand zu hoch"; the renderer stays usable. Survives zza_load. */
void          zza_set_work_limit(zza_renderer*, uint64_t ops);
/* Operations charged by the last zza_render / zza_anim_begin / zza_anim_frame call. */
uint64_t      zza_work_used(const zza_renderer*);
const char*   zza_error(const zza_renderer*);
size_t        zza_mem_used(const zza_renderer*);
size_t        zza_mem_peak(const zza_renderer*);

#ifdef ZZA_TEST_HOOKS
/* ---- test only (compiled with ZZA_TEST_HOOKS; used by the Go test hooks, never by the firmware) ---- */
int zza_test_alloc(zza_renderer*, size_t n);                              /* allocate+free n bytes via budget */
void zza_test_set_limit(zza_renderer*, size_t limit);                     /* change the budget limit */
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
#endif /* ZZA_TEST_HOOKS */

#ifdef __cplusplus
}
#endif
