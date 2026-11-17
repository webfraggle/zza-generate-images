// Strict check of train data JSON before ArduinoJson parses it. ArduinoJson
// is lenient (trailing text, unquoted keys, single quotes, raw control
// characters, numbers beyond float64); the Go server rejects all of these
// (json.Unmarshal), so the device must too.
#pragma once
#include <cstddef>
#include <string>

namespace zza {

// Containers nested deeper than this are rejected (device limit).
constexpr int kMaxDataDepth = 32;

// Returns true if data[0..len) is one RFC 8259 JSON value (surrounded by
// whitespace only) whose numbers fit a float64 and whose nesting is at most
// kMaxDataDepth; otherwise false with err set. Linear time, no allocation
// except for long number tokens, no recursion.
bool checkDataJson(const char* data, size_t len, std::string& err);

}  // namespace zza
