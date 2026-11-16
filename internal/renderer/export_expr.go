package renderer

// EvalIntExprForTest exposes the coordinate-expression evaluator so the C++
// port in device/zzarender can be checked against Go with a shared corpus.
func EvalIntExprForTest(expr string, vars map[string]int) (int, error) {
	return evalIntExpr(expr, vars)
}
