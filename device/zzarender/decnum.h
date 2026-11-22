// Bounded canonical form of a JSON number token, so that numbers of any
// length are checked and converted in linear time with a fixed buffer.
// Used by jsoncheck (float64 range) and the ArduinoJson ZZA PATCH (value of
// tokens longer than its 63-character buffer).
#pragma once
#include <cstddef>
#include <cstdio>

namespace zza {

// Significant digits that make strtod of the canonical form round exactly
// like strtod of the whole token: a halfway point between two doubles has at
// most 767 significant digits, so the kept digits plus a sticky digit lie on
// the same side of it as the full value.
constexpr int kDecExactDigits = 800;

// Bytes a DecNorm buffer needs beyond maxSig: sign, sticky digit, "e",
// exponent and NUL.
constexpr int kDecExtra = 32;

// DecNorm reads a JSON number token (RFC 8259: -?(0|[1-9]\d*)(\.\d+)?([eE][+-]?\d+)?)
// one character at a time, checks its syntax and keeps at most maxSig
// significant digits in a caller-provided buffer of maxSig + kDecExtra bytes;
// finish() writes "[-]D…D[1]e<exp>" (a sticky '1' if a dropped digit was not
// zero), which has the same value class for strtod as the whole token.
class DecNorm {
 public:
  DecNorm(char* buf, int maxSig)
      : buf_(buf), maxSig_(maxSig), n_(0), state_(kStart), neg_(false), sticky_(false), sig_(false),
        pointExp_(0), exp_(0), expNeg_(false) {}

  // Feeds the next character; false on a syntax error (the token stays invalid).
  bool feed(char c) {
    bool digit = c >= '0' && c <= '9';
    switch (state_) {
      case kStart:
        if (c == '-' && !neg_) {
          neg_ = true;
          return true;
        }
        if (!digit) return bad();
        state_ = c == '0' ? kZero : kInt;
        mantissa(c, false);
        return true;
      case kZero:
      case kInt:
        if (digit && state_ == kInt) {
          mantissa(c, false);
          return true;
        }
        if (c == '.') {
          state_ = kDot;
          return true;
        }
        if (c == 'e' || c == 'E') {
          state_ = kE;
          return true;
        }
        return bad();
      case kDot:
      case kFrac:
        if (digit) {
          state_ = kFrac;
          mantissa(c, true);
          return true;
        }
        if (state_ == kFrac && (c == 'e' || c == 'E')) {
          state_ = kE;
          return true;
        }
        return bad();
      case kE:
        if (c == '+' || c == '-') {
          expNeg_ = c == '-';
          state_ = kESign;
          return true;
        }
        [[fallthrough]];
      case kESign:
      case kExp:
        if (!digit) return bad();
        state_ = kExp;
        if (exp_ < 1000000000LL) exp_ = exp_ * 10 + (c - '0');  // saturates far beyond float64
        return true;
      case kBad:
        return false;
    }
    return bad();
  }

  // True if the characters so far form a complete number.
  bool complete() const { return state_ == kZero || state_ == kInt || state_ == kFrac || state_ == kExp; }

  // True if the value is zero (no significant digit).
  bool zero() const { return !sig_; }

  // Decimal exponent e of the first significant digit: 10^e <= |value| < 10^(e+1).
  long long leadExp() const { return pointExp_ + (expNeg_ ? -exp_ : exp_) - 1; }

  // Writes the canonical form into the buffer and returns it (complete() only).
  const char* finish() {
    size_t i = 0;
    if (neg_) buf_[i++] = '-';
    if (!sig_) {
      buf_[i++] = '0';
      buf_[i] = 0;
      return buf_;
    }
    for (int k = 0; k < n_; k++) buf_[i++] = buf_[kOff + k];
    long long kept = n_;
    if (sticky_) {
      buf_[i++] = '1';
      kept++;
    }
    std::snprintf(buf_ + i, (size_t)kDecExtra - 4, "e%lld", pointExp_ + (expNeg_ ? -exp_ : exp_) - kept);
    return buf_;
  }

 private:
  enum State { kStart, kZero, kInt, kDot, kFrac, kE, kESign, kExp, kBad };
  // Digits are stored from kOff so that finish() can move them left behind the sign.
  static constexpr int kOff = 1;

  bool bad() {
    state_ = kBad;
    return false;
  }

  void mantissa(char c, bool frac) {
    if (!sig_ && c == '0') {
      if (frac) pointExp_--;  // 0.00d: the first significant digit moves right
      return;
    }
    sig_ = true;
    if (!frac) pointExp_++;
    if (n_ < maxSig_) {
      buf_[kOff + n_++] = c;
    } else if (c != '0') {
      sticky_ = true;
    }
  }

  char* buf_;
  int maxSig_, n_;
  State state_;
  bool neg_, sticky_, sig_;
  long long pointExp_, exp_;
  bool expNeg_;
};

}  // namespace zza
