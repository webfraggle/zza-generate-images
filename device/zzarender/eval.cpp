#include "eval.h"

#include <algorithm>
#include <cmath>
#include <cstdint>
#include <cstdio>
#include <cstring>
#include <utility>

#include "numfmt.h"
#include "utf8.h"

namespace zza {

namespace {

constexpr int kMaxPipelineDepth = 4;
constexpr int kMaxCondDepth = 10;
constexpr double kMaxFormatOffsetSec = 3.2e9;

const char* const kWeekdayLong[7] = {"Sonntag", "Montag", "Dienstag", "Mittwoch",
                                     "Donnerstag", "Freitag", "Samstag"};
const char* const kWeekdayShort[7] = {"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"};

// ---- time helpers (proleptic Gregorian, no time zones) ----

int64_t daysFromCivil(int64_t y, int m, int d) {
  y -= m <= 2;
  const int64_t era = (y >= 0 ? y : y - 399) / 400;
  const int64_t yoe = y - era * 400;
  const int64_t doy = (153 * (m + (m > 2 ? -3 : 9)) + 2) / 5 + d - 1;
  const int64_t doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
  return era * 146097 + doe - 719468;
}

void civilFromDays(int64_t z, int* y, int* m, int* d) {
  z += 719468;
  const int64_t era = (z >= 0 ? z : z - 146096) / 146097;
  const int64_t doe = z - era * 146097;
  const int64_t yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
  const int64_t yy = yoe + era * 400;
  const int64_t doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
  const int64_t mp = (5 * doy + 2) / 153;
  *d = static_cast<int>(doy - (153 * mp + 2) / 5 + 1);
  *m = static_cast<int>(mp < 10 ? mp + 3 : mp - 9);
  *y = static_cast<int>(yy + (*m <= 2));
}

int weekdayOfDays(int64_t days) {
  int64_t w = (days + 4) % 7;  // 1970-01-01 was a Thursday
  return static_cast<int>(w < 0 ? w + 7 : w);
}

std::string pad(int64_t v, int width) {
  char buf[32];
  snprintf(buf, sizeof buf, "%0*lld", width, static_cast<long long>(v));
  return buf;
}

// strings.ReplaceAll for a non-empty `from`.
std::string replaceAll(std::string_view s, std::string_view from, std::string_view to) {
  std::string out;
  size_t pos = 0;
  for (;;) {
    size_t i = s.find(from, pos);
    if (i == std::string_view::npos) break;
    out.append(s.data() + pos, i - pos);
    out.append(to);
    pos = i + from.size();
  }
  out.append(s.data() + pos, s.size() - pos);
  return out;
}

// Token replacement of formatTime; the tokens apply one after the other (like Go).
std::string formatTime(const zza_time& t, int64_t offsetSec, std::string_view pattern) {
  int64_t days = daysFromCivil(t.year, t.month, t.day);
  int64_t secs = days * 86400 + t.hour * 3600 + t.minute * 60 + t.second + offsetSec;
  int64_t d2 = secs / 86400;
  int64_t rem = secs % 86400;
  if (rem < 0) {
    rem += 86400;
    d2--;
  }
  int y, mo, d;
  civilFromDays(d2, &y, &mo, &d);
  int hour = static_cast<int>(rem / 3600), minute = static_cast<int>(rem % 3600 / 60),
      second = static_cast<int>(rem % 60);
  int wd = weekdayOfDays(d2);
  int h12 = hour % 12;
  if (h12 == 0) h12 = 12;
  std::string r(pattern);
  r = replaceAll(r, "EEEE", kWeekdayLong[wd]);
  r = replaceAll(r, "yyyy", pad(y, 4));
  r = replaceAll(r, "HH", pad(hour, 2));
  r = replaceAll(r, "hh", pad(h12, 2));
  r = replaceAll(r, "MM", pad(mo, 2));
  r = replaceAll(r, "mm", pad(minute, 2));
  r = replaceAll(r, "ss", pad(second, 2));
  r = replaceAll(r, "dd", pad(d, 2));
  r = replaceAll(r, "EE", kWeekdayShort[wd]);
  return r;
}

// ---- text helpers ----

// Splits at sep outside of '...', "..." and (...). A surplus ")" is a normal
// character. Parts are not trimmed.
void splitTopLevel(std::string_view s, char sep, std::vector<std::string_view>& parts) {
  int depth = 0;
  char quote = 0;
  size_t start = 0;
  for (size_t i = 0; i < s.size(); i++) {
    char ch = s[i];
    if (quote) {
      if (ch == quote) quote = 0;
      continue;
    }
    if (ch == '\'' || ch == '"') quote = ch;
    else if (ch == '(') depth++;
    else if (ch == ')') {
      if (depth > 0) depth--;
    } else if (ch == sep && depth == 0) {
      parts.push_back(s.substr(start, i - start));
      start = i + 1;
    }
  }
  parts.push_back(s.substr(start));
}

// Trimmed non-empty comma-separated parts.
void parseArgs(std::string_view s, std::vector<std::string_view>& args) {
  std::vector<std::string_view> parts;
  splitTopLevel(s, ',', parts);
  for (auto p : parts) {
    p = trimSpace(p);
    if (!p.empty()) args.push_back(p);
  }
}

std::string_view unquote(std::string_view s) {
  s = trimSpace(s);
  if (s.size() >= 2 && ((s.front() == '\'' && s.back() == '\'') || (s.front() == '"' && s.back() == '"')))
    return s.substr(1, s.size() - 2);
  return s;
}

bool hasPrefix(std::string_view s, std::string_view p) { return s.substr(0, p.size()) == p; }
bool hasSuffix(std::string_view s, std::string_view p) {
  return s.size() >= p.size() && s.substr(s.size() - p.size()) == p;
}

// `^[+-]?(\d+\.?\d*|\.\d+)$`
bool isNumLiteral(std::string_view t) {
  size_t i = 0;
  if (i < t.size() && (t[i] == '+' || t[i] == '-')) i++;
  auto dig = [&](size_t k) { return k < t.size() && t[k] >= '0' && t[k] <= '9'; };
  if (dig(i)) {
    while (dig(i)) i++;
    if (i < t.size() && t[i] == '.') i++;
    while (dig(i)) i++;
    return i == t.size();
  }
  if (i < t.size() && t[i] == '.') {
    i++;
    if (!dig(i)) return false;
    while (dig(i)) i++;
    return i == t.size();
  }
  return false;
}

// Removes every open...close span (open and close included), restarting the
// search after each removal like Go's doStripBetween, but without rescanning
// the unchanged prefix. Both delimiters empty would loop forever in Go; the
// string is returned unchanged there.
std::string stripBetween(std::string_view s, std::string_view open, std::string_view close) {
  if (open.empty() && close.empty()) return std::string(s);
  std::string out;  // finished prefix of the current string (no open occurrence in it)
  size_t pos = 0;   // current string = out + s[pos:]
  for (;;) {
    // Earliest occurrence of `open` in out + s[pos:]: spanning the join first.
    size_t startInOut = std::string::npos;  // index in `out` where a spanning match starts
    size_t consumedFromS = 0;               // bytes of s[pos:] covered by the open match
    if (open.size() > 1) {
      size_t maxK = std::min(open.size() - 1, out.size());
      for (size_t k = maxK; k >= 1; k--) {  // longest overlap = earliest start
        if (out.compare(out.size() - k, k, open.data(), k) == 0 &&
            s.substr(pos, open.size() - k) == open.substr(k)) {
          startInOut = out.size() - k;
          consumedFromS = open.size() - k;
          break;
        }
      }
    }
    size_t closeFrom;  // index in s where the close search starts
    size_t cutStartInS = 0;
    if (startInOut != std::string::npos) {
      closeFrom = pos + consumedFromS;
    } else {
      size_t i = s.find(open, pos);
      if (i == std::string_view::npos) break;
      cutStartInS = i;
      closeFrom = i + open.size();
    }
    size_t e = s.find(close, closeFrom);
    if (e == std::string_view::npos) break;
    if (startInOut != std::string::npos) {
      out.resize(startInOut);
    } else {
      out.append(s.data() + pos, cutStartInS - pos);
    }
    pos = e + close.size();
  }
  out.append(s.data() + pos, s.size() - pos);
  return out;
}

// doMath: non-numeric value -> unchanged; non-numeric arg -> "" if strict else unchanged.
std::string doMath(const std::string& value, const std::string& arg, bool strictArg, char op) {
  double v, a;
  if (!parseFloat(trimSpace(value), &v)) return value;
  if (!parseFloat(trimSpace(arg), &a)) return strictArg ? std::string() : value;
  double r;
  switch (op) {
    case '*': r = v * a; break;
    case '/': r = a == 0 ? 0 : v / a; break;
    case '%': {
      if (a == 0) {
        r = 0;
      } else {
        r = std::fmod(v, a);
        if (r != 0 && (r < 0) != (a < 0)) r += a;
        if (r == a) r = 0;
      }
      break;
    }
    case '+': r = v + a; break;
    default: r = v - a; break;
  }
  if (!std::isinf(r) && !std::isnan(r) && r == std::trunc(r) && r >= -9223372036854775808.0 &&
      r <= 9223372036854775807.0) {
    // 2^63 itself is platform-dependent in Go; saturate.
    long long iv = (r >= 9223372036854775807.0) ? 9223372036854775807LL : static_cast<long long>(r);
    return std::to_string(iv);
  }
  return formatF(r);
}

// Go fmt %v of a decoded JSON value.
void fmtValue(JsonVariantConst v, std::string& out, int depth) {
  if (v.isNull()) {
    out += "<nil>";
  } else if (v.is<bool>()) {
    out += v.as<bool>() ? "true" : "false";
  } else if (v.is<const char*>()) {
    out += v.as<const char*>();
  } else if (v.is<JsonArrayConst>()) {
    out.push_back('[');
    bool first = true;
    for (JsonVariantConst e : v.as<JsonArrayConst>()) {
      if (!first) out.push_back(' ');
      first = false;
      if (depth < 64) fmtValue(e, out, depth + 1);
    }
    out.push_back(']');
  } else if (v.is<JsonObjectConst>()) {
    std::vector<std::pair<const char*, JsonVariantConst>> kv;
    for (JsonPairConst p : v.as<JsonObjectConst>()) kv.emplace_back(p.key().c_str(), p.value());
    std::sort(kv.begin(), kv.end(),
              [](const auto& a, const auto& b) { return strcmp(a.first, b.first) < 0; });
    out += "map[";
    bool first = true;
    for (auto& e : kv) {
      if (!first) out.push_back(' ');
      first = false;
      out += e.first;
      out.push_back(':');
      if (depth < 64) fmtValue(e.second, out, depth + 1);
    }
    out.push_back(']');
  } else if (v.is<double>()) {
    out += formatV(v.as<double>());
  }
}

}  // namespace

Eval::Eval(JsonVariantConst data, const zza_time& now)
    : data_(data), now_(now), animFrame_(0), animFrames_(1), animDelayCs_(0), regex_(nullptr) {
  zza::setIntVar(intVars_, "anim.frame", 0);
  zza::setIntVar(intVars_, "anim.frames", 1);
}

void Eval::setAnim(int frame, int frames, int delayCs) {
  animFrame_ = frame;
  animFrames_ = frames;
  animDelayCs_ = delayCs;
  zza::setIntVar(intVars_, "anim.frame", frame);
  zza::setIntVar(intVars_, "anim.frames", frames);
}

void Eval::setStrVar(std::string_view name, std::string_view value) {
  for (auto& v : strVars_) {
    if (v.name == name) {
      v.value.assign(value);
      return;
    }
  }
  strVars_.push_back(StrVar{std::string(name), std::string(value)});
}

void Eval::setIntVar(std::string_view name, int value) { zza::setIntVar(intVars_, name, value); }

int Eval::evalInt(const std::string& expr, bool* ok) const { return evalIntExpr(expr, intVars_, ok); }

// ---- interpolation ----

std::string Eval::interpolate(const std::string& s) const {
  if (s.find("{{") == std::string::npos) return s;
  // Port of the Go regexp `\{\{((?:'[^']*'|"[^"]*"|[^}])+)\}\}` with its
  // leftmost-first semantics. g[p] = index of the closing "}}" when continuing
  // the repeated group at p, or -1. Alternatives in order: 'quoted', "quoted",
  // any char but '}', then the end "}}".
  const size_t n = s.size();
  std::vector<int32_t> g(n + 1, -1);
  std::vector<int32_t> first(n + 1, -1);  // same as g but needs at least one item
  int64_t nextS = -1, nextD = -1;
  for (size_t pp = n; pp-- > 0;) {
    const char c = s[pp];
    int32_t item = -1;
    if (c == '\'' && nextS >= 0) item = g[static_cast<size_t>(nextS) + 1];
    if (item < 0 && c == '"' && nextD >= 0) item = g[static_cast<size_t>(nextD) + 1];
    if (item < 0 && c != '}') item = g[pp + 1];
    first[pp] = item;
    if (item < 0 && c == '}' && pp + 1 < n && s[pp + 1] == '}') g[pp] = static_cast<int32_t>(pp);
    else g[pp] = item;
    if (c == '\'') nextS = static_cast<int64_t>(pp);
    if (c == '"') nextD = static_cast<int64_t>(pp);
  }

  std::string out;
  size_t last = 0, i = 0;
  for (;;) {
    size_t j = s.find("{{", i);
    if (j == std::string::npos) break;
    size_t p = j + 2;
    int32_t close = p < n ? first[p] : -1;
    if (close < 0) {
      i = j + 1;
      continue;
    }
    std::string_view inner = trimSpace(std::string_view(s).substr(p, static_cast<size_t>(close) - p));
    out.append(s, last, j - last);
    out += pipeline(inner, 0);
    last = static_cast<size_t>(close) + 2;
    i = last;
  }
  out.append(s, last, std::string::npos);
  return out;
}

std::string Eval::pipeline(std::string_view expr, int depth) const {
  if (depth > kMaxPipelineDepth) return std::string();
  std::vector<std::string_view> stages;
  splitTopLevel(expr, '|', stages);
  std::string value = resolveVar(trimSpace(stages[0]));
  for (size_t i = 1; i < stages.size(); i++) value = applyFilter(std::move(value), trimSpace(stages[i]), depth);
  return value;
}

std::string Eval::resolveVar(std::string_view name) const {
  if (name == "now" || hasPrefix(name, "now.")) return resolveNow(name);
  if (name == "anim" || hasPrefix(name, "anim.")) return resolveAnim(name);
  return resolvePath(name);
}

std::string Eval::resolveAnim(std::string_view name) const {
  if (name == "anim.frame") return std::to_string(animFrame_);
  if (name == "anim.frames") return std::to_string(animFrames_);
  if (name == "anim.time")
    return formatF(static_cast<double>(static_cast<int64_t>(animFrame_) * animDelayCs_) / 100);
  return std::string();
}

std::string Eval::resolveNow(std::string_view name) const {
  if (name == "now") return pad(now_.hour, 2) + ":" + pad(now_.minute, 2);
  if (name == "now.hour") return std::to_string(now_.hour);
  if (name == "now.hour12") {
    int h = now_.hour % 12;
    return std::to_string(h == 0 ? 12 : h);
  }
  if (name == "now.minute") return std::to_string(now_.minute);
  if (name == "now.second") return std::to_string(now_.second);
  if (name == "now.day") return std::to_string(now_.day);
  if (name == "now.month") return std::to_string(now_.month);
  if (name == "now.year") return std::to_string(now_.year);
  if (name == "now.weekday")
    return kWeekdayLong[weekdayOfDays(daysFromCivil(now_.year, now_.month, now_.day))];
  return std::string();
}

std::string Eval::resolvePath(std::string_view path) const {
  std::vector<std::string_view> parts;
  size_t start = 0;
  for (;;) {  // strings.Split(path, ".")
    size_t i = path.find('.', start);
    if (i == std::string_view::npos) {
      parts.push_back(path.substr(start));
      break;
    }
    parts.push_back(path.substr(start, i - start));
    start = i + 1;
  }
  std::string_view p0 = trimSpace(parts[0]);
  for (const auto& sv : strVars_) {
    if (sv.name == p0) return parts.size() == 1 ? sv.value : std::string();
  }
  JsonVariantConst cur = data_;
  for (auto part : parts) {
    if (!cur.is<JsonObjectConst>()) return std::string();
    std::string key(trimSpace(part));
    JsonObjectConst obj = cur.as<JsonObjectConst>();
    if (!obj.containsKey(key.c_str())) return std::string();
    cur = obj[key.c_str()];
  }
  if (cur.isNull()) return std::string();
  std::string out;
  fmtValue(cur, out, 0);
  return out;
}

std::string Eval::numericArg(std::string_view arg, int depth, bool* fromExpr) const {
  std::string_view t = trimSpace(arg);
  if (t.size() >= 2 && ((t.front() == '\'' && t.back() == '\'') || (t.front() == '"' && t.back() == '"'))) {
    *fromExpr = false;
    return std::string(unquote(t));
  }
  if (isNumLiteral(t)) {
    *fromExpr = false;
    return std::string(t);
  }
  *fromExpr = true;
  return pipeline(t, depth + 1);
}

std::string Eval::applyFilter(std::string value, std::string_view filter, int depth) const {
  // parseFilterCall
  filter = trimSpace(filter);
  std::string_view name = filter;
  std::vector<std::string_view> args;
  size_t paren = filter.find('(');
  if (paren != std::string_view::npos && hasSuffix(filter, ")")) {
    name = trimSpace(filter.substr(0, paren));
    parseArgs(filter.substr(paren + 1, filter.size() - paren - 2), args);
  }
  const size_t na = args.size();
  auto mathOp = [&](char op) -> std::string {
    bool strict;
    std::string arg = numericArg(args[0], depth, &strict);
    return doMath(value, arg, strict, op);
  };

  if (name == "strip") {
    if (na == 1) {
      std::string_view pre = unquote(args[0]);
      if (hasPrefix(value, pre)) value.erase(0, pre.size());
      return value;
    }
  } else if (name == "stripAll") {
    if (na == 1) {
      std::string_view from = unquote(args[0]);
      if (from.empty()) return value;
      return replaceAll(value, from, "");
    }
  } else if (name == "stripBetween") {
    if (na == 2) return stripBetween(value, unquote(args[0]), unquote(args[1]));
  } else if (name == "before") {
    if (na == 1) {
      std::string_view sep = unquote(args[0]);
      if (!sep.empty()) {
        size_t i = value.find(sep);
        if (i != std::string::npos) value.resize(i);
      }
      return value;
    }
  } else if (name == "after") {
    if (na == 1) {
      std::string_view sep = unquote(args[0]);
      if (!sep.empty()) {
        size_t i = value.find(sep);
        if (i != std::string::npos) return value.substr(i + sep.size());
      }
      return std::string();
    }
  } else if (name == "upper") {
    return toUpper(value);
  } else if (name == "lower") {
    return toLower(value);
  } else if (name == "trim") {
    return std::string(trimSpace(value));
  } else if (name == "prefix") {
    if (na == 1) return std::string(unquote(args[0])) + value;
  } else if (name == "suffix") {
    if (na == 1) return value + std::string(unquote(args[0]));
  } else if (name == "format") {
    if (na == 1) return formatTime(now_, 0, unquote(args[0]));
    if (na == 2) {
      int64_t off = 0;
      bool strict;
      std::string arg = numericArg(args[1], depth, &strict);
      double sec;
      if (parseFloat(trimSpace(arg), &sec) && !std::isnan(sec) && std::fabs(sec) <= kMaxFormatOffsetSec) {
        // time.Duration(sec * 1e9) truncates; Second() then floors.
        int64_t ns = static_cast<int64_t>(sec * 1e9);
        off = ns / 1000000000;
        if (ns % 1000000000 < 0) off--;
      }
      return formatTime(now_, off, unquote(args[0]));
    }
  } else if (name == "clock") {
    double v;
    if (!parseFloat(trimSpace(value), &v) || std::isnan(v) || std::isinf(v)) return std::string();
    double m = std::fmod(std::floor(v), 1440);
    if (m < 0) m += 1440;
    int mi = static_cast<int>(m);
    return pad(mi / 60, 2) + ":" + pad(mi % 60, 2);
  } else if (name == "replace") {
    if (na == 2) {
      std::string_view from = unquote(args[0]);
      if (from.empty()) return value;
      std::string_view to = unquote(args[1]);
      size_t count = 0;
      for (size_t i = value.find(from); i != std::string::npos; i = value.find(from, i + from.size())) count++;
      // len(value) + count*(len(to)-len(from)) > limit, in signed 64-bit like Go.
      int64_t total = static_cast<int64_t>(value.size()) +
                      static_cast<int64_t>(count) * (static_cast<int64_t>(to.size()) - static_cast<int64_t>(from.size()));
      if (total > static_cast<int64_t>(kMaxFilterValueLen)) return value;
      return replaceAll(value, from, to);
    }
  } else if (name == "match") {
    if (na == 2) {
      bool strict;
      std::string a = numericArg(args[1], depth, &strict);
      long long n;
      int ni = -1;
      if (parseInt64(trimSpace(a), &n)) ni = n > 0x7fffffff ? 0x7fffffff : (n < -1 ? -1 : static_cast<int>(n));
      // Go matchGroup: limits first, then the compiled pattern (no program for
      // the pattern = invalid pattern in Go).
      std::string_view pattern = unquote(args[0]);
      if (ni < 0 || runeCount(pattern) > kMaxMatchPatternLen || value.size() > kMaxFilterValueLen)
        return std::string();
      const RegexProg* prog = regex_ ? regex_->find(pattern) : nullptr;
      if (!prog) return std::string();
      return matchGroup(*prog, value, ni);
    }
  } else if (name == "mul") {
    if (na == 1) return mathOp('*');
  } else if (name == "div") {
    if (na == 1) return mathOp('/');
  } else if (name == "mod") {
    if (na == 1) return mathOp('%');
  } else if (name == "add") {
    if (na == 1) return mathOp('+');
  } else if (name == "sub") {
    if (na == 1) return mathOp('-');
  } else if (name == "minutes") {
    // ^(\d{1,2}):(\d{1,2}) on the trimmed value
    std::string_view v = trimSpace(value);
    auto dig = [&](size_t k) { return k < v.size() && v[k] >= '0' && v[k] <= '9'; };
    size_t i = 0;
    while (i < 2 && dig(i)) i++;
    if (i >= 1 && i < v.size() && v[i] == ':' && i <= 2) {
      int h = 0;
      for (size_t k = 0; k < i; k++) h = h * 10 + (v[k] - '0');
      size_t j = i + 1, s2 = j;
      int mi = 0;
      while (j - s2 < 2 && dig(j)) mi = mi * 10 + (v[j++] - '0');
      if (j > s2) return std::to_string(h * 60 + mi);
    }
    return std::string();
  } else if (name == "round") {
    double v;
    if (parseFloat(trimSpace(value), &v)) {
      double r = std::round(v);
      if (!std::isinf(r) && !std::isnan(r) && r >= -9223372036854775808.0 && r <= 9223372036854775807.0) {
        long long iv = (r >= 9223372036854775807.0) ? 9223372036854775807LL : static_cast<long long>(r);
        return std::to_string(iv);
      }
    }
  }
  return value;
}

// ---- conditions ----

bool Eval::cond(const std::string& expr) const { return evalCond(expr, 0); }

bool Eval::evalCond(std::string_view expr, int depth) const {
  if (depth > kMaxCondDepth) return false;
  expr = trimSpace(expr);
  if (expr.empty()) return true;
  if (hasPrefix(expr, "not(") && hasSuffix(expr, ")")) return !evalCond(expr.substr(4, expr.size() - 5), depth + 1);

  size_t paren = expr.find('(');
  if (paren == std::string_view::npos || !hasSuffix(expr, ")")) return false;
  std::string_view fn = trimSpace(expr.substr(0, paren));
  std::vector<std::string_view> args;
  parseArgs(expr.substr(paren + 1, expr.size() - paren - 2), args);
  if (args.empty()) return false;

  if (fn == "and") {
    for (auto a : args)
      if (!evalCond(a, depth + 1)) return false;
    return true;
  }
  if (fn == "or") {
    for (auto a : args)
      if (evalCond(a, depth + 1)) return true;
    return false;
  }

  std::string field = pipeline(trimSpace(args[0]), 1);
  if (fn == "startsWith") {
    if (args.size() == 2) return hasPrefix(field, unquote(args[1]));
  } else if (fn == "endsWith") {
    if (args.size() == 2) return hasSuffix(field, unquote(args[1]));
  } else if (fn == "contains") {
    if (args.size() == 2) return field.find(unquote(args[1])) != std::string::npos;
  } else if (fn == "isEmpty") {
    return field.empty();
  } else if (fn == "eq" || fn == "equals") {
    if (args.size() == 2) return field == unquote(args[1]);
  } else if (fn == "greaterThan") {
    if (args.size() == 2) {
      double fv, th;
      return parseFloat(field, &fv) && parseFloat(trimSpace(unquote(args[1])), &th) && fv > th;
    }
  }
  return false;
}

}  // namespace zza
