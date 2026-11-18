#include "numfmt.h"

#include <cmath>
#include <cstdio>
#include <cstdlib>
#include <cstring>

#include "decnum.h"

namespace zza {

namespace {

// Decimal strings at least this long go through DecNorm (the data parser's
// threshold: ArduinoJson's number buffer holds 63 characters).
constexpr size_t kLongNumber = 64;

// strtod of a syntactically valid decimal float (Go syntax, no underscores),
// in bounded work: the digits are fed to DecNorm as 0.<digits>e<exp+intDigits>,
// which keeps kDecExactDigits significant digits plus a sticky digit, so the
// result rounds exactly like strtod of the whole string (and like the data
// parser, which uses the same canonical form).
double strtodLong(std::string_view s) {
  std::string buf((size_t)kDecExactDigits + kDecExtra, '\0');  // heap: > 512 bytes
  DecNorm norm(&buf[0], kDecExactDigits);
  size_t i = 0;
  if (s[i] == '+' || s[i] == '-') {
    if (s[i] == '-') norm.feed('-');
    i++;
  }
  norm.feed('0');
  norm.feed('.');
  long long intDigits = 0;
  bool frac = false;
  for (; i < s.size() && s[i] != 'e' && s[i] != 'E'; i++) {
    if (s[i] == '.') {
      frac = true;
      continue;
    }
    if (!frac && intDigits < 1000000000LL) intDigits++;
    norm.feed(s[i]);
  }
  long long e = 0;
  bool eneg = false;
  if (i < s.size()) {  // exponent (validated by the caller)
    i++;
    if (s[i] == '+' || s[i] == '-') eneg = s[i++] == '-';
    for (; i < s.size(); i++)
      if (e < 1000000000LL) e = e * 10 + (s[i] - '0');  // saturates far beyond float64
  }
  long long total = (eneg ? -e : e) + intDigits;
  char exp[24];
  snprintf(exp, sizeof exp, "e%lld", total);
  for (const char* p = exp; *p; p++) norm.feed(*p);
  return strtod(norm.finish(), nullptr);
}

struct Digits {
  char d[24];  // decimal digits, no leading/trailing zeros (except the value 0)
  int nd = 0;
  int dp = 0;  // decimal point position: value = 0.d * 10^dp
};

// Shortest digit string that round-trips to v (v finite, > 0): the shortest
// correctly rounded "%.{p}e" that parses back to v.
Digits shortest(double v) {
  Digits r;
  char buf[48];
  for (int p = 0; p <= 17; p++) {
    snprintf(buf, sizeof buf, "%.*e", p, v);
    if (p == 17 || strtod(buf, nullptr) == v) break;
  }
  // buf = d[.ddd]e[+-]XX
  const char* e = strchr(buf, 'e');
  int exp10 = atoi(e + 1);
  for (const char* q = buf; q < e; q++)
    if (*q >= '0' && *q <= '9') r.d[r.nd++] = *q;
  while (r.nd > 1 && r.d[r.nd - 1] == '0') r.nd--;
  r.dp = exp10 + 1;
  return r;
}

std::string special(double v, bool* done) {
  *done = true;
  if (std::isnan(v)) return "NaN";
  if (std::isinf(v)) return v < 0 ? "-Inf" : "+Inf";
  if (v == 0) return std::signbit(v) ? "-0" : "0";
  *done = false;
  return std::string();
}

}  // namespace

std::string formatF(double v) {
  bool done;
  std::string s = special(v, &done);
  if (done) return s;
  Digits g = shortest(std::fabs(v));
  if (v < 0) s.push_back('-');
  if (g.dp <= 0) {
    s += "0.";
    s.append(static_cast<size_t>(-g.dp), '0');
    s.append(g.d, static_cast<size_t>(g.nd));
  } else if (g.dp >= g.nd) {
    s.append(g.d, static_cast<size_t>(g.nd));
    s.append(static_cast<size_t>(g.dp - g.nd), '0');
  } else {
    s.append(g.d, static_cast<size_t>(g.dp));
    s.push_back('.');
    s.append(g.d + g.dp, static_cast<size_t>(g.nd - g.dp));
  }
  return s;
}

std::string formatV(double v) {
  bool done;
  std::string s = special(v, &done);
  if (done) return s;
  Digits g = shortest(std::fabs(v));
  int exp = g.dp - 1;
  // %v is %g with shortest digits: %e form when the exponent is < -4 or >= 6
  // (verified against Go: 1e6 -> 1e+06, 100000 -> 100000).
  if (v < 0) s.push_back('-');
  if (exp < -4 || exp >= 6) {
    s.push_back(g.d[0]);
    if (g.nd > 1) {
      s.push_back('.');
      s.append(g.d + 1, static_cast<size_t>(g.nd - 1));
    }
    char eb[16];
    snprintf(eb, sizeof eb, "e%c%02d", exp < 0 ? '-' : '+', exp < 0 ? -exp : exp);
    s += eb;
    return s;
  }
  std::string f = formatF(std::fabs(v));
  s += f;
  return s;
}

namespace {

bool isDigit(char c) { return c >= '0' && c <= '9'; }
bool isHex(char c) { return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F'); }
char lower(char c) { return (c >= 'A' && c <= 'Z') ? static_cast<char>(c + 32) : c; }

bool ieq(std::string_view s, const char* lit) {
  size_t n = strlen(lit);
  if (s.size() != n) return false;
  for (size_t i = 0; i < n; i++)
    if (lower(s[i]) != lit[i]) return false;
  return true;
}

// Port of strconv's underscoreOK: underscores only between digits or between
// a base prefix and a digit. A leading sign is skipped.
bool underscoreOK(std::string_view s) {
  char saw = '^';
  size_t i = 0;
  if (!s.empty() && (s[0] == '-' || s[0] == '+')) s.remove_prefix(1);
  bool hex = false;
  if (s.size() >= 2 && s[0] == '0' && (lower(s[1]) == 'b' || lower(s[1]) == 'o' || lower(s[1]) == 'x')) {
    i = 2;
    saw = '0';
    hex = lower(s[1]) == 'x';
  }
  for (; i < s.size(); i++) {
    char c = s[i];
    if (isDigit(c) || (hex && lower(c) >= 'a' && lower(c) <= 'f')) {
      saw = '0';
      continue;
    }
    if (c == '_') {
      if (saw != '0') return false;
      saw = '_';
      continue;
    }
    if (saw == '_') return false;
    saw = '!';
  }
  return saw != '_';
}

}  // namespace

bool parseFloat(std::string_view s, double* out) {
  if (s.empty()) return false;
  size_t i = 0;
  bool neg = false;
  if (s[0] == '+' || s[0] == '-') {
    neg = s[0] == '-';
    i = 1;
  }
  std::string_view rest = s.substr(i);
  if (ieq(rest, "inf") || ieq(rest, "infinity")) {
    *out = neg ? -INFINITY : INFINITY;
    return true;
  }
  if (i == 0 && ieq(rest, "nan")) {
    *out = NAN;
    return true;
  }
  // readFloat
  bool hex = false;
  char expChar = 'e';
  if (i + 2 < s.size() && s[i] == '0' && lower(s[i + 1]) == 'x') {
    hex = true;
    expChar = 'p';
    i += 2;
  }
  bool underscores = false, sawdot = false, sawdigits = false;
  for (; i < s.size(); i++) {
    char c = s[i];
    if (c == '_') {
      underscores = true;
    } else if (c == '.') {
      if (sawdot) break;
      sawdot = true;
    } else if (isDigit(c) || (hex && isHex(c))) {
      sawdigits = true;
    } else {
      break;
    }
  }
  if (!sawdigits) return false;
  if (i < s.size() && lower(s[i]) == expChar) {
    i++;
    if (i >= s.size()) return false;
    if (s[i] == '+' || s[i] == '-') i++;
    if (i >= s.size() || !isDigit(s[i])) return false;
    for (; i < s.size() && (isDigit(s[i]) || s[i] == '_'); i++)
      if (s[i] == '_') underscores = true;
  } else if (hex) {
    return false;
  }
  if (i != s.size()) return false;
  if (underscores && !underscoreOK(s)) return false;

  char stackBuf[kLongNumber];
  std::string heapBuf;
  std::string_view num = s;
  if (underscores) {
    heapBuf.reserve(s.size());
    for (char c : s)
      if (c != '_') heapBuf.push_back(c);
    num = heapBuf;
  }
  double v;
  if (num.size() >= kLongNumber) {
    if (hex) {
      std::string h(num);  // hex digits: strtod is linear and exact
      v = strtod(h.c_str(), nullptr);
    } else {
      v = strtodLong(num);
    }
  } else {
    memcpy(stackBuf, num.data(), num.size());
    stackBuf[num.size()] = 0;
    v = strtod(stackBuf, nullptr);
  }
  if (std::isinf(v)) return false;  // out of range
  *out = v;
  return true;
}

bool parseInt64(std::string_view s, long long* out) {
  size_t i = 0;
  bool neg = false;
  if (!s.empty() && (s[0] == '+' || s[0] == '-')) {
    neg = s[0] == '-';
    i = 1;
  }
  if (i >= s.size()) return false;
  unsigned long long v = 0;
  const unsigned long long limit = neg ? 9223372036854775808ULL : 9223372036854775807ULL;
  for (; i < s.size(); i++) {
    if (!isDigit(s[i])) return false;
    unsigned d = static_cast<unsigned>(s[i] - '0');
    if (v > (limit - d) / 10) return false;
    v = v * 10 + d;
  }
  *out = neg ? static_cast<long long>(0ULL - v) : static_cast<long long>(v);
  return true;
}

}  // namespace zza
