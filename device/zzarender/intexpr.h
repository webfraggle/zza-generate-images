// Integer coordinate expressions: port of internal/renderer/expr.go.
#pragma once
#include <cstdint>
#include <string>
#include <string_view>
#include <vector>

namespace zza {

struct IntVar {
  std::string name;
  int value;
};
// Few entries (loop index, anim.frame, ...): a linear vector beats a map.
using IntVars = std::vector<IntVar>;

// Sets or adds a variable.
void setIntVar(IntVars& vars, std::string_view name, int value);

// Evaluates + - * / ( ) unary minus, integer literals and variables
// (letters, digits, '_', '.'). Arithmetic is 64-bit with wrap-around like Go's
// int; division is truncating, division by zero is an error. Returns false on
// any error (syntax, undefined variable, nesting > 50).
bool evalIntExpr64(std::string_view expr, const IntVars& vars, int64_t* out);

// Same, but the result must fit in int (ok=false otherwise).
int evalIntExpr(const std::string& expr, const IntVars& vars, bool* ok);

}  // namespace zza
