// cgo compiles only files in the package directory; this unity file pulls in
// the C++ core from device/zzarender (master copy).
#include "anim.cpp"
#include "canvas.cpp"
#include "capi.cpp"
#include "eval.cpp"
#include "intexpr.cpp"
#include "jsoncheck.cpp"
#include "layers.cpp"
#include "numfmt.cpp"
#include "package.cpp"
#include "regex.cpp"
#include "sha256.cpp"
#include "splitflap.cpp"
#include "text.cpp"
#include "utf8.cpp"

#ifdef ZZA_TEST_HOOKS
// The core's ArduinoJson lives in its own namespace (ZZA PATCH), so it cannot
// clash with another copy in the firmware. The build breaks without it.
static_assert(sizeof(ArduinoJson::ArduinoJson6216_ZZA::JsonDocument) > 0,
              "ArduinoJson must be in the ArduinoJson6216_ZZA namespace (ZZA PATCH)");
#endif
