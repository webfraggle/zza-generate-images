#include "jsoncheck.h"

#include <cmath>
#include <cstdlib>
#include <string>

#include "decnum.h"

namespace zza {

namespace {

constexpr int kRangeDigits = 330;

bool jsWs(char c) { return c == ' ' || c == '\t' || c == '\n' || c == '\r'; }
bool jsDigit(char c) { return c >= '0' && c <= '9'; }
bool jsHex(char c) { return jsDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F'); }

struct Scan {
  const char* p;
  const char* end;
  std::string* err;

  bool fail(const char* what) {
    *err = std::string("invalid data JSON: ") + what;
    return false;
  }
  void ws() {
    while (p < end && jsWs(*p)) p++;
  }
  // A string starting at '"'. Bytes >= 0x20 are taken as they are (Go
  // replaces invalid UTF-8 instead of rejecting it).
  bool str() {
    p++;  // opening quote
    while (p < end) {
      unsigned char c = (unsigned char)*p++;
      if (c == '"') return true;
      if (c < 0x20) return fail("control character in string");
      if (c != '\\') continue;
      if (p >= end) break;
      char e = *p++;
      if (e == 'u') {
        for (int i = 0; i < 4; i++, p++)
          if (p >= end || !jsHex(*p)) return fail("invalid \\u escape");
      } else if (e != '"' && e != '\\' && e != '/' && e != 'b' && e != 'f' && e != 'n' && e != 'r' && e != 't') {
        return fail("invalid escape");
      }
    }
    return fail("unterminated string");
  }
  // A number; Go decodes it into a float64 and rejects values beyond its
  // range (underflow gives 0). Decided from the decimal exponent; only a
  // first digit at 10^308 needs strtod, on a bounded canonical copy: the
  // overflow threshold (between DBL_MAX and 2^1024) is an integer of 309
  // digits, so 330 significant digits plus a sticky digit decide exactly.
  // Not inlined: the buffer stays out of checkDataJson's frame (<= 512 bytes).
  __attribute__((noinline)) bool num() {
    char buf[kRangeDigits + kDecExtra];  // 362 bytes
    DecNorm norm(buf, kRangeDigits);
    while (p < end && (jsDigit(*p) || *p == '-' || *p == '+' || *p == '.' || *p == 'e' || *p == 'E')) {
      if (!norm.feed(*p++)) return fail("invalid number");
    }
    if (!norm.complete()) return fail("invalid number");
    if (norm.zero() || norm.leadExp() < 308) return true;
    if (norm.leadExp() > 308 || std::isinf(std::strtod(norm.finish(), nullptr))) return fail("number out of range");
    return true;
  }
  bool lit(const char* w) {
    for (; *w; w++, p++)
      if (p >= end || *p != *w) return fail("invalid literal");
    return true;
  }
};

}  // namespace

bool checkDataJson(const char* data, size_t len, std::string& err) {
  Scan s{data, data + len, &err};
  char stack[kMaxDataDepth];  // '{' or '['
  int depth = 0;
  // After a value inside a container: ',' or the closing bracket follows.
  for (;;) {
    s.ws();
    if (s.p >= s.end) return s.fail("unexpected end");
    char c = *s.p;
    // One value.
    if (c == '{' || c == '[') {
      if (depth == kMaxDataDepth) return s.fail("nested deeper than 32");
      stack[depth++] = c;
      s.p++;
      s.ws();
      if (s.p < s.end && *s.p == (c == '{' ? '}' : ']')) {
        s.p++;
        depth--;
      } else if (c == '{') {
        if (s.p >= s.end || *s.p != '"') return s.fail("expected object key");
        if (!s.str()) return false;
        s.ws();
        if (s.p >= s.end || *s.p != ':') return s.fail("expected ':'");
        s.p++;
        continue;  // the member's value
      } else {
        continue;  // the first element
      }
    } else if (c == '"') {
      if (!s.str()) return false;
    } else if (c == '-' || jsDigit(c)) {
      if (!s.num()) return false;
    } else if (c == 't') {
      if (!s.lit("true")) return false;
    } else if (c == 'f') {
      if (!s.lit("false")) return false;
    } else if (c == 'n') {
      if (!s.lit("null")) return false;
    } else {
      return s.fail("unexpected character");
    }
    // Close containers or continue with the next member / element.
    for (;;) {
      s.ws();
      if (depth == 0) {
        if (s.p != s.end) return s.fail("text after the value");
        return true;
      }
      if (s.p >= s.end) return s.fail("unexpected end");
      char open = stack[depth - 1];
      if (*s.p == (open == '{' ? '}' : ']')) {
        s.p++;
        depth--;
        continue;
      }
      if (*s.p != ',') return s.fail("expected ',' or end of container");
      s.p++;
      if (open == '{') {
        s.ws();
        if (s.p >= s.end || *s.p != '"') return s.fail("expected object key");
        if (!s.str()) return false;
        s.ws();
        if (s.p >= s.end || *s.p != ':') return s.fail("expected ':'");
        s.p++;
      }
      break;  // next value
    }
  }
}

}  // namespace zza
