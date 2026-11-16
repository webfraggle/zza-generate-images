// UTF-8 / Latin-1 helpers with Go semantics (strings.ToUpper/ToLower/TrimSpace,
// utf8.DecodeRune). Invalid bytes decode as U+FFFD with width 1, like Go.
#pragma once
#include <cstddef>
#include <cstdint>
#include <string>
#include <string_view>

namespace zza {

constexpr uint32_t kRuneError = 0xFFFD;

// Decodes one rune at s[0..n). Returns the width (>=1; 1 with kRuneError for
// invalid input; 0 only if n == 0).
int decodeRune(const char* s, size_t n, uint32_t* r);

// Appends r as UTF-8 (invalid runes, i.e. surrogates or > U+10FFFF, become U+FFFD).
void appendRune(std::string& out, uint32_t r);

// Number of runes (invalid bytes count as one rune each), like utf8.RuneCountInString.
size_t runeCount(std::string_view s);

// strings.ToUpper / strings.ToLower (simple Unicode mapping per rune).
std::string toUpper(std::string_view s);
std::string toLower(std::string_view s);

// unicode.IsSpace.
bool isSpaceRune(uint32_t r);

// strings.TrimSpace (as view into s).
std::string_view trimSpace(std::string_view s);

// UTF-8 -> Latin-1 bytes. Runes above U+00FF (and invalid bytes) become '?'.
// If strict is set, returns false instead on the first rune above U+00FF.
bool utf8ToLatin1(std::string_view in, std::string& out, bool strict);

// Latin-1 bytes -> UTF-8.
std::string latin1ToUtf8(std::string_view in);

}  // namespace zza
