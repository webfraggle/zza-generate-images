package renderer

import "time"

// EvalForTest evaluates expr with the Go reference evaluator: as a condition
// ("true"/"false") when isCond is set, otherwise as an interpolated string.
// It exists so the C++ evaluator in device/zzarender can be checked against
// Go with a shared input corpus; the renderer itself does not use it.
func EvalForTest(data map[string]any, now time.Time, frame, frames, delayCS int, expr string, isCond bool) string {
	e := newEvaluatorAt(data, now).withAnim(frame, frames, delayCS)
	if isCond {
		if e.EvalCondition(expr) {
			return "true"
		}
		return "false"
	}
	return e.Interpolate(expr)
}

// MatchGroupForTest exposes the match filter's core (limits, regexp
// compilation, group selection) so the C++ regex engine can be checked
// against Go; the renderer itself does not use it.
func MatchGroupForTest(value, pattern string, n int) string {
	return matchGroup(value, pattern, n)
}
