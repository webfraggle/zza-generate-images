// RE2-compatible regex for the `match` filter. Patterns are compiled by the Go
// exporter (internal/devexport.CompileRegex, regexp/syntax) into programs; the
// core runs them in a Pike VM (linear in the input, leftmost-first like Go's
// regexp, no backtracking).
#pragma once
#include <cstddef>
#include <cstdint>
#include <string>
#include <string_view>

#include "mem.h"
#include "third_party/ArduinoJson.h"

namespace zza {

// Go internal/renderer: maxMatchPatternLen (runes) and maxFilterValueLen (bytes;
// the match input and the replace result).
constexpr size_t kMaxMatchPatternLen = 100;
constexpr size_t kMaxFilterValueLen = 64 * 1024;

// A compiled program. JSON form (devexport.Prog):
//   [start, numCap, [[op,out,arg,[r0,r1,…]], …]]
// with Go's regexp/syntax.InstOp values. InstRune carries sorted, disjoint
// [lo,hi] pairs, InstRune1 exactly one rune (case folding already expanded).
class RegexProg {
 public:
  explicit RegexProg(ZzaBudget* budget);

  // Loads and validates a program (all indices in range, known ops, sorted
  // rune ranges). Returns false for a malformed program and leaves it empty.
  // Throws std::bad_alloc when the budget is exhausted.
  bool load(JsonArrayConst prog);
  bool empty() const { return inst_.empty(); }

 private:
  struct Inst {
    uint8_t op;
    uint32_t out;
    uint32_t arg;
    uint32_t runeOff;  // into runes_
    uint32_t runeLen;
  };

  bool loadInsts(JsonArrayConst insts);
  bool matchRune(const Inst& in, int32_t r) const;
  void clear();

  friend class RegexVM;
  friend std::string matchGroup(const RegexProg&, std::string_view, int);

  ZzaBudget* budget_;
  uint32_t start_;
  uint32_t ncap_;
  ZzaVec<Inst> inst_;
  ZzaVec<int32_t> runes_;
};

// Capture group `group` (0 = whole match) of the first match in utf8Input, or
// "" on no match, missing group, group < 0 or input longer than
// kMaxFilterValueLen (Go matchGroup). Invalid UTF-8 bytes are U+FFFD like Go.
// Scratch memory comes from the program's budget (std::bad_alloc if exhausted).
std::string matchGroup(const RegexProg& prog, std::string_view utf8Input, int group);

// Supplies the program for a pattern string (nullptr: none available, which
// the evaluator treats like an invalid pattern in Go).
class RegexSource {
 public:
  virtual const RegexProg* find(std::string_view pattern) const = 0;

 protected:
  ~RegexSource() = default;
};

// Pattern -> program table, loaded from a JSON object {"<pattern>": <prog>, …}
// (the package's `_re` table).
class RegexTable : public RegexSource {
 public:
  explicit RegexTable(ZzaBudget* budget);
  // False if any program is malformed (the table is left empty).
  bool load(JsonObjectConst table);
  const RegexProg* find(std::string_view pattern) const override;

 private:
  ZzaBudget* budget_;
  ZzaVec<char> names_;       // all patterns, concatenated
  ZzaVec<uint32_t> nameOff_;  // pattern i = names_[nameOff_[i], nameOff_[i+1])
  ZzaVec<RegexProg> progs_;
};

}  // namespace zza
