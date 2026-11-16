#include "intexpr.h"

#include <climits>

#include "numfmt.h"

namespace zza {

void setIntVar(IntVars& vars, std::string_view name, int value) {
  for (auto& v : vars) {
    if (v.name == name) {
      v.value = value;
      return;
    }
  }
  vars.push_back(IntVar{std::string(name), value});
}

namespace {

constexpr int kMaxExprDepth = 50;

// unicode.IsLetter(rune(b)) for a single byte b (Latin-1 range).
bool isLatin1Letter(unsigned char c) {
  if ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) return true;
  return c == 0xAA || c == 0xB5 || c == 0xBA || (c >= 0xC0 && c <= 0xD6) ||
         (c >= 0xD8 && c <= 0xF6) || (c >= 0xF8);
}
bool isAsciiDigit(char c) { return c >= '0' && c <= '9'; }

// Wrap-around 64-bit arithmetic without UB.
int64_t wrapAdd(int64_t a, int64_t b) { return static_cast<int64_t>(static_cast<uint64_t>(a) + static_cast<uint64_t>(b)); }
int64_t wrapSub(int64_t a, int64_t b) { return static_cast<int64_t>(static_cast<uint64_t>(a) - static_cast<uint64_t>(b)); }
int64_t wrapMul(int64_t a, int64_t b) { return static_cast<int64_t>(static_cast<uint64_t>(a) * static_cast<uint64_t>(b)); }

struct Parser {
  std::string_view in;
  size_t pos = 0;
  int depth = 0;
  const IntVars* vars;

  void skipSpace() {
    while (pos < in.size() && in[pos] == ' ') pos++;
  }

  bool expr(int64_t* out) {
    int64_t val;
    if (!term(&val)) return false;
    for (;;) {
      skipSpace();
      if (pos >= in.size()) break;
      char op = in[pos];
      if (op != '+' && op != '-') break;
      pos++;
      skipSpace();
      int64_t right;
      if (!term(&right)) return false;
      val = (op == '+') ? wrapAdd(val, right) : wrapSub(val, right);
    }
    *out = val;
    return true;
  }

  bool term(int64_t* out) {
    int64_t val;
    if (!factor(&val)) return false;
    for (;;) {
      skipSpace();
      if (pos >= in.size()) break;
      char op = in[pos];
      if (op != '*' && op != '/') break;
      pos++;
      skipSpace();
      int64_t right;
      if (!factor(&right)) return false;
      if (op == '*') {
        val = wrapMul(val, right);
      } else {
        if (right == 0) return false;
        if (right == -1) val = wrapSub(0, val);  // INT64_MIN / -1 wraps in Go
        else val /= right;
      }
    }
    *out = val;
    return true;
  }

  bool factor(int64_t* out) {
    skipSpace();
    if (pos >= in.size()) return false;
    char ch = in[pos];
    if (ch == '-') {
      if (++depth > kMaxExprDepth) return false;
      pos++;
      int64_t v;
      bool ok = factor(&v);
      depth--;
      if (!ok) return false;
      *out = wrapSub(0, v);
      return true;
    }
    if (ch == '(') {
      if (++depth > kMaxExprDepth) return false;
      pos++;
      int64_t v;
      bool ok = expr(&v);
      depth--;
      if (!ok) return false;
      skipSpace();
      if (pos >= in.size() || in[pos] != ')') return false;
      pos++;
      *out = v;
      return true;
    }
    if (isAsciiDigit(ch)) {
      size_t start = pos;
      while (pos < in.size() && isAsciiDigit(in[pos])) pos++;
      long long n;
      if (!parseInt64(in.substr(start, pos - start), &n)) return false;
      *out = n;
      return true;
    }
    if (isLatin1Letter(static_cast<unsigned char>(ch)) || ch == '_') {
      size_t start = pos;
      while (pos < in.size()) {
        char c = in[pos];
        if (isLatin1Letter(static_cast<unsigned char>(c)) || isAsciiDigit(c) || c == '_' || c == '.') pos++;
        else break;
      }
      std::string_view name = in.substr(start, pos - start);
      for (const auto& v : *vars) {
        if (v.name == name) {
          *out = v.value;
          return true;
        }
      }
      return false;
    }
    return false;
  }
};

}  // namespace

bool evalIntExpr64(std::string_view expr, const IntVars& vars, int64_t* out) {
  Parser p;
  p.in = expr;
  p.vars = &vars;
  int64_t v;
  if (!p.expr(&v)) return false;
  p.skipSpace();
  if (p.pos < p.in.size()) return false;
  *out = v;
  return true;
}

int evalIntExpr(const std::string& expr, const IntVars& vars, bool* ok) {
  int64_t v = 0;
  bool good = evalIntExpr64(expr, vars, &v) && v >= INT_MIN && v <= INT_MAX;
  if (ok) *ok = good;
  return good ? static_cast<int>(v) : 0;
}

}  // namespace zza
