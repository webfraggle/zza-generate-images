// Template evaluator: {{...}} interpolation, filter pipelines and conditions.
// Port of internal/renderer/evaluator.go; output must match Go byte for byte.
#pragma once
#include <string>
#include <string_view>
#include <vector>

#include "intexpr.h"
#include "regex.h"
#include "third_party/ArduinoJson.h"
#include "zzarender_c.h"

namespace zza {

class Eval {
 public:
  // `data` must stay valid for the lifetime of the Eval (it is not copied).
  Eval(JsonVariantConst data, const zza_time& now);

  // {{anim.frame}}, {{anim.frames}}, {{anim.time}} and the coordinate variables
  // anim.frame / anim.frames. Default: frame 0, frames 1, delay 0.
  void setAnim(int frame, int frames, int delayCs);

  // Loop/flap-scope variables. A string variable shadows the data key of the
  // same name (like Go's withLoopVars); an int variable is visible to
  // coordinate expressions. Copy the Eval to get a child scope.
  void setStrVar(std::string_view name, std::string_view value);
  void setIntVar(std::string_view name, int value);
  const IntVars& intVars() const { return intVars_; }

  // Programs for `match` patterns (not owned; must outlive the Eval). Without a
  // source, or for a pattern it does not know, match yields "" like an
  // invalid pattern in Go.
  void setRegexSource(const RegexSource* src) { regex_ = src; }

  // Replaces every {{expr}} in s.
  std::string interpolate(const std::string& s) const;
  // Condition (startsWith, endsWith, contains, isEmpty, eq, equals,
  // greaterThan, not, and, or). Empty expression is true.
  bool cond(const std::string& expr) const;
  // Coordinate expression with this scope's integer variables.
  int evalInt(const std::string& expr, bool* ok) const;

 private:
  struct StrVar {
    std::string name;
    std::string value;
  };

  std::string pipeline(std::string_view expr, int depth) const;
  std::string resolveVar(std::string_view name) const;
  std::string resolveNow(std::string_view name) const;
  std::string resolveAnim(std::string_view name) const;
  std::string resolvePath(std::string_view path) const;
  std::string applyFilter(std::string value, std::string_view filter, int depth) const;
  std::string numericArg(std::string_view arg, int depth, bool* fromExpr) const;
  bool evalCond(std::string_view expr, int depth) const;

  JsonVariantConst data_;
  zza_time now_;
  int animFrame_, animFrames_, animDelayCs_;
  std::vector<StrVar> strVars_;
  IntVars intVars_;
  const RegexSource* regex_;
};

}  // namespace zza
