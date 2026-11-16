// Go-compatible number formatting and parsing (strconv.FormatFloat, fmt %v,
// strconv.ParseFloat, strconv.Atoi). Used by the evaluator so output matches
// the Go renderer byte for byte.
#pragma once
#include <string>
#include <string_view>

namespace zza {

// strconv.FormatFloat(v, 'f', -1, 64): shortest digits that round-trip, no exponent.
std::string formatF(double v);

// fmt.Sprint(float64) / %v: shortest digits; %e form (e+06) when the decimal
// exponent is < -4 or >= 6 (verified against Go; 1e6 prints as 1e+06).
std::string formatV(double v);

// strconv.ParseFloat(s, 64): true only if it returns no error (so Inf from
// overflow and malformed input are false). Accepts inf/infinity/nan, hex
// floats and Go underscore syntax like Go does. No trimming.
bool parseFloat(std::string_view s, double* out);

// strconv.Atoi: optional sign and decimal digits, no spaces, must fit int64.
bool parseInt64(std::string_view s, long long* out);

}  // namespace zza
