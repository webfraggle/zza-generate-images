// Package loader: reads a ZZA package (see internal/devexport/doc.go) from
// memory, verifies it and decodes it into budget memory. Nothing of the input
// is retained after load(), so callers may free their buffers right away.
#pragma once
#include <cstdint>
#include <memory>
#include <string>

#include "mem.h"
#include "regex.h"
#include "third_party/ArduinoJson.h"
#include "zzarender_c.h"

namespace zza {

// ArduinoJson allocator drawing from the budget. The block size is kept in a
// 16-byte header because the library's deallocate() does not pass it.
struct JsonAlloc {
  ZzaBudget* b;
  JsonAlloc() : b(nullptr) {}
  explicit JsonAlloc(ZzaBudget* bb) : b(bb) {}
  void* allocate(size_t n);
  void deallocate(void* ptr);
  void* reallocate(void*, size_t) { throw std::bad_alloc(); }  // never used (no shrinkToFit)
};
using BudgetJsonDoc = BasicJsonDocument<JsonAlloc>;

// Upper bound for the pool of a document parsed from d[0..n).
size_t jsonCapacity(const uint8_t* d, size_t n);

// Decoded image: w*h RGBA8888 pixels, non-premultiplied, row-major.
struct Img {
  uint16_t w;
  uint16_t h;
  ZzaBytes px;
  explicit Img(ZzaBudget* b) : w(0), h(0), px(ZzaAlloc<uint8_t>(b)) {}
};

struct Glyph {
  int32_t adv;    // natural 26.6
  int16_t bx, by; // mask top-left relative to pen at baseline
  uint16_t w, h;  // mask size; w*h == 0: no mask
  uint32_t off;   // into Font::masks (decoded A8, w*h bytes)
  uint8_t cp;
};

struct KernEnt {
  uint16_t key;  // left<<8 | right
  int32_t kern;  // natural 26.6
};

// Decoded glyph set. Glyph masks are always stored raw (RLE is expanded on load).
struct Font {
  int16_t ascent, descent, height;
  ZzaVec<Glyph> glyphs;  // sorted by cp
  ZzaVec<KernEnt> kern;  // sorted by key
  ZzaBytes masks;
  explicit Font(ZzaBudget* b)
      : ascent(0), descent(0), height(0), glyphs(ZzaAlloc<Glyph>(b)), kern(ZzaAlloc<KernEnt>(b)),
        masks(ZzaAlloc<uint8_t>(b)) {}

  const Glyph* find(unsigned cp) const;               // nullptr if absent
  int32_t kernFor(unsigned left, unsigned right) const;  // 0 if the pair is not in the table
  const uint8_t* mask(const Glyph& g) const { return masks.data() + g.off; }
};

class Package {
 public:
  explicit Package(ZzaBudget* budget);
  ~Package();
  Package(const Package&) = delete;
  Package& operator=(const Package&) = delete;

  // Loads a package. Returns ZZA_OK, or ZZA_ERR_FORMAT with err naming the
  // offending file. Throws std::bad_alloc when the budget is exhausted. Call
  // once per Package; after a failure discard the Package.
  int load(const zza_file* files, int n, std::string& err);

  // Look up by id ("i0", "f1"). nullptr if absent.
  const Img* image(const char* id) const;
  const Font* font(const char* id) const;
  // Compiled program of a constant match pattern (key of t.json "_re"); nullptr if absent.
  const RegexProg* regex(const std::string& pattern) const;
  // All programs of "_re", for the evaluator's match filter.
  const RegexSource* regexSource() const;
  // The template (t.json root); null variant before a successful load.
  JsonVariantConst tmpl() const;
  // The template name of the manifest ("name"; "" if absent), seeds animations.
  const std::string& name() const;
  // The manifest's "clock" as ZZA_CLOCK_* (ZZA_CLOCK_NONE if absent or no package).
  int clock() const;

 private:
  struct Impl;
  std::unique_ptr<Impl> d_;
};

}  // namespace zza
