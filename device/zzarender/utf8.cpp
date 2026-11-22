#include "utf8.h"

#include "casetab.h"

namespace zza {

int decodeRune(const char* sp, size_t n, uint32_t* r) {
  const unsigned char* s = reinterpret_cast<const unsigned char*>(sp);
  if (n == 0) {
    *r = kRuneError;
    return 0;
  }
  unsigned c0 = s[0];
  if (c0 < 0x80) {
    *r = c0;
    return 1;
  }
  auto bad = [&]() {
    *r = kRuneError;
    return 1;
  };
  if (c0 < 0xC2 || c0 > 0xF4) return bad();
  if (c0 < 0xE0) {  // 2 bytes
    if (n < 2 || (s[1] & 0xC0) != 0x80) return bad();
    *r = ((c0 & 0x1F) << 6) | (s[1] & 0x3F);
    return 2;
  }
  if (c0 < 0xF0) {  // 3 bytes
    if (n < 3) return bad();
    unsigned lo = 0x80, hi = 0xBF;
    if (c0 == 0xE0) lo = 0xA0;       // no overlongs
    else if (c0 == 0xED) hi = 0x9F;  // no surrogates
    if (s[1] < lo || s[1] > hi || (s[2] & 0xC0) != 0x80) return bad();
    *r = ((c0 & 0x0F) << 12) | ((s[1] & 0x3F) << 6) | (s[2] & 0x3F);
    return 3;
  }
  if (n < 4) return bad();  // 4 bytes
  unsigned lo = 0x80, hi = 0xBF;
  if (c0 == 0xF0) lo = 0x90;
  else if (c0 == 0xF4) hi = 0x8F;
  if (s[1] < lo || s[1] > hi || (s[2] & 0xC0) != 0x80 || (s[3] & 0xC0) != 0x80) return bad();
  *r = ((c0 & 0x07) << 18) | ((s[1] & 0x3F) << 12) | ((s[2] & 0x3F) << 6) | (s[3] & 0x3F);
  return 4;
}

void appendRune(std::string& out, uint32_t r) {
  if (r > 0x10FFFF || (r >= 0xD800 && r <= 0xDFFF)) r = kRuneError;
  if (r < 0x80) {
    out.push_back(static_cast<char>(r));
  } else if (r < 0x800) {
    out.push_back(static_cast<char>(0xC0 | (r >> 6)));
    out.push_back(static_cast<char>(0x80 | (r & 0x3F)));
  } else if (r < 0x10000) {
    out.push_back(static_cast<char>(0xE0 | (r >> 12)));
    out.push_back(static_cast<char>(0x80 | ((r >> 6) & 0x3F)));
    out.push_back(static_cast<char>(0x80 | (r & 0x3F)));
  } else {
    out.push_back(static_cast<char>(0xF0 | (r >> 18)));
    out.push_back(static_cast<char>(0x80 | ((r >> 12) & 0x3F)));
    out.push_back(static_cast<char>(0x80 | ((r >> 6) & 0x3F)));
    out.push_back(static_cast<char>(0x80 | (r & 0x3F)));
  }
}

size_t runeCount(std::string_view s) {
  size_t n = 0;
  for (size_t i = 0; i < s.size();) {
    uint32_t r;
    i += static_cast<size_t>(decodeRune(s.data() + i, s.size() - i, &r));
    n++;
  }
  return n;
}

// unicode.To for the upper (toLower=false) or lower case.
static uint32_t mapCase(uint32_t r, bool toLower) {
  if (r < 0x80) {
    if (!toLower) return (r >= 'a' && r <= 'z') ? r - 32 : r;
    return (r >= 'A' && r <= 'Z') ? r + 32 : r;
  }
  size_t lo = 0, hi = sizeof(kCaseRanges) / sizeof(kCaseRanges[0]);
  while (lo < hi) {
    size_t m = lo + (hi - lo) / 2;
    const CaseRange& c = kCaseRanges[m];
    if (c.lo <= r && r <= c.hi) {
      int32_t d = toLower ? c.dLower : c.dUpper;
      if (d == kCaseUpperLower) {
        // Even offsets from lo are upper case, odd ones lower case.
        return c.lo + (((r - c.lo) & ~1u) | (toLower ? 1u : 0u));
      }
      return static_cast<uint32_t>(static_cast<int64_t>(r) + d);
    }
    if (r < c.lo) hi = m;
    else lo = m + 1;
  }
  return r;
}

// strings.Map(unicode.ToUpper/ToLower, s): invalid bytes become U+FFFD.
static std::string mapString(std::string_view s, bool toLower) {
  std::string out;
  out.reserve(s.size());
  for (size_t i = 0; i < s.size();) {
    unsigned char c = static_cast<unsigned char>(s[i]);
    if (c < 0x80) {
      out.push_back(static_cast<char>(mapCase(c, toLower)));
      i++;
      continue;
    }
    uint32_t r;
    int w = decodeRune(s.data() + i, s.size() - i, &r);
    appendRune(out, mapCase(r, toLower));
    i += static_cast<size_t>(w);
  }
  return out;
}

std::string toUpper(std::string_view s) { return mapString(s, false); }
std::string toLower(std::string_view s) { return mapString(s, true); }

bool isSpaceRune(uint32_t r) {
  if (r <= 0xFF) return r == ' ' || (r >= '\t' && r <= '\r') || r == 0x85 || r == 0xA0;
  return r == 0x1680 || (r >= 0x2000 && r <= 0x200A) || r == 0x2028 || r == 0x2029 ||
         r == 0x202F || r == 0x205F || r == 0x3000;
}

std::string_view trimSpace(std::string_view s) {
  size_t b = 0;
  while (b < s.size()) {
    uint32_t r;
    int w = decodeRune(s.data() + b, s.size() - b, &r);
    if (!isSpaceRune(r) || (r == kRuneError && w == 1)) break;
    b += static_cast<size_t>(w);
  }
  size_t e = s.size();
  while (e > b) {
    // Step back to the start of the last rune (at most 4 bytes).
    size_t st = e - 1;
    size_t lim = (e - b > 4) ? e - 4 : b;
    while (st > lim && (static_cast<unsigned char>(s[st]) & 0xC0) == 0x80) st--;
    uint32_t r;
    int w = decodeRune(s.data() + st, e - st, &r);
    if (st + static_cast<size_t>(w) != e) {  // not a valid rune ending here: last byte alone
      st = e - 1;
      r = kRuneError;
    }
    if (!isSpaceRune(r)) break;
    e = st;
  }
  return s.substr(b, e - b);
}

bool utf8ToLatin1(std::string_view in, std::string& out, bool strict) {
  out.clear();
  out.reserve(in.size());
  for (size_t i = 0; i < in.size();) {
    uint32_t r;
    int w = decodeRune(in.data() + i, in.size() - i, &r);
    i += static_cast<size_t>(w);
    if (r > 0xFF) {
      if (strict) return false;
      r = '?';
    }
    out.push_back(static_cast<char>(r));
  }
  return true;
}

std::string latin1ToUtf8(std::string_view in) {
  std::string out;
  out.reserve(in.size() + in.size() / 4);
  for (unsigned char c : in) appendRune(out, c);
  return out;
}

}  // namespace zza
