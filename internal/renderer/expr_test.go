package renderer

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// --- evalIntExpr ---

func TestEvalIntExpr_Literal(t *testing.T) {
	v, err := evalIntExpr("42", nil)
	if err != nil || v != 42 {
		t.Errorf("got %d, err %v; want 42", v, err)
	}
}

func TestEvalIntExpr_Addition(t *testing.T) {
	v, err := evalIntExpr("10 + 5", nil)
	if err != nil || v != 15 {
		t.Errorf("got %d, err %v; want 15", v, err)
	}
}

func TestEvalIntExpr_Subtraction(t *testing.T) {
	v, err := evalIntExpr("10 - 3", nil)
	if err != nil || v != 7 {
		t.Errorf("got %d, err %v; want 7", v, err)
	}
}

func TestEvalIntExpr_Multiplication(t *testing.T) {
	v, err := evalIntExpr("3 * 7", nil)
	if err != nil || v != 21 {
		t.Errorf("got %d, err %v; want 21", v, err)
	}
}

func TestEvalIntExpr_Division(t *testing.T) {
	v, err := evalIntExpr("20 / 4", nil)
	if err != nil || v != 5 {
		t.Errorf("got %d, err %v; want 5", v, err)
	}
}

func TestEvalIntExpr_IntegerDivision(t *testing.T) {
	// 7 / 2 = 3 (integer division)
	v, err := evalIntExpr("7 / 2", nil)
	if err != nil || v != 3 {
		t.Errorf("got %d, err %v; want 3", v, err)
	}
}

func TestEvalIntExpr_Precedence(t *testing.T) {
	// 2 + 3 * 4 = 14 (not 20)
	v, err := evalIntExpr("2 + 3 * 4", nil)
	if err != nil || v != 14 {
		t.Errorf("got %d, err %v; want 14", v, err)
	}
}

func TestEvalIntExpr_Parentheses(t *testing.T) {
	// (2 + 3) * 4 = 20
	v, err := evalIntExpr("(2 + 3) * 4", nil)
	if err != nil || v != 20 {
		t.Errorf("got %d, err %v; want 20", v, err)
	}
}

func TestEvalIntExpr_UnaryMinus(t *testing.T) {
	v, err := evalIntExpr("-5", nil)
	if err != nil || v != -5 {
		t.Errorf("got %d, err %v; want -5", v, err)
	}
}

func TestEvalIntExpr_Variable(t *testing.T) {
	vars := map[string]int{"i": 3}
	v, err := evalIntExpr("i * 20 + 10", vars)
	if err != nil || v != 70 {
		t.Errorf("got %d, err %v; want 70", v, err)
	}
}

func TestEvalIntExpr_LoopIndex(t *testing.T) {
	vars := map[string]int{"loop.index": 2}
	v, err := evalIntExpr("loop.index * 12 + 30", vars)
	if err != nil || v != 54 {
		t.Errorf("got %d, err %v; want 54", v, err)
	}
}

func TestEvalIntExpr_UsersExample(t *testing.T) {
	// (i-1)*20+10 with i=3 → (3-1)*20+10 = 50
	vars := map[string]int{"i": 3}
	v, err := evalIntExpr("(i-1)*20+10", vars)
	if err != nil || v != 50 {
		t.Errorf("got %d, err %v; want 50", v, err)
	}
}

func TestEvalIntExpr_Error_DivisionByZero(t *testing.T) {
	_, err := evalIntExpr("10 / 0", nil)
	if err == nil {
		t.Error("expected error for division by zero")
	}
}

func TestEvalIntExpr_Error_UndefinedVar(t *testing.T) {
	_, err := evalIntExpr("x + 1", nil)
	if err == nil {
		t.Error("expected error for undefined variable")
	}
}

func TestEvalIntExpr_Error_UnclosedParen(t *testing.T) {
	_, err := evalIntExpr("(1 + 2", nil)
	if err == nil {
		t.Error("expected error for unclosed parenthesis")
	}
}

func TestEvalIntExpr_Error_Empty(t *testing.T) {
	_, err := evalIntExpr("", nil)
	if err == nil {
		t.Error("expected error for empty expression")
	}
}

func TestEvalIntExpr_Error_TrailingGarbage(t *testing.T) {
	_, err := evalIntExpr("42 foo", nil)
	if err == nil {
		t.Error("expected error for trailing garbage")
	}
}

func TestEvalIntExpr_Error_DepthLimit(t *testing.T) {
	// 51 unary minuses → exceeds maxExprDepth (50)
	expr := ""
	for i := 0; i < maxExprDepth+1; i++ {
		expr += "-"
	}
	expr += "1"
	_, err := evalIntExpr(expr, nil)
	if err == nil {
		t.Errorf("expected error for depth > %d", maxExprDepth)
	}
}

func TestIntOrExpr_NegativePlainInt(t *testing.T) {
	// YAML negative integer (e.g. x: -5) — strconv.Atoi("-5") succeeds
	type wrapper struct {
		V IntOrExpr `yaml:"v"`
	}
	var w wrapper
	if err := yaml.Unmarshal([]byte("v: -5"), &w); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if got := w.V.Resolve(nil); got != -5 {
		t.Errorf("got %d, want -5", got)
	}
}

// --- IntOrExpr YAML unmarshaling ---

func parseIntOrExpr(t *testing.T, yamlStr string) IntOrExpr {
	t.Helper()
	type wrapper struct {
		V IntOrExpr `yaml:"v"`
	}
	var w wrapper
	if err := yaml.Unmarshal([]byte("v: "+yamlStr), &w); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	return w.V
}

func TestIntOrExpr_PlainInt(t *testing.T) {
	ie := parseIntOrExpr(t, "45")
	if got := ie.Resolve(nil); got != 45 {
		t.Errorf("got %d, want 45", got)
	}
}

func TestIntOrExpr_Zero(t *testing.T) {
	ie := parseIntOrExpr(t, "0")
	if got := ie.Resolve(nil); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestIntOrExpr_Expression(t *testing.T) {
	ie := parseIntOrExpr(t, `"{{i * 20 + 10}}"`)
	e := NewEvaluator(nil)
	e.intVars = map[string]int{"i": 2}
	if got := ie.Resolve(e); got != 50 {
		t.Errorf("got %d, want 50", got)
	}
}

func TestIntOrExpr_ExpressionNilEval(t *testing.T) {
	ie := parseIntOrExpr(t, `"{{i * 20}}"`)
	// nil eval → returns 0 (no panic)
	if got := ie.Resolve(nil); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestIntOrExpr_Error_InvalidString(t *testing.T) {
	type wrapper struct {
		V IntOrExpr `yaml:"v"`
	}
	var w wrapper
	err := yaml.Unmarshal([]byte(`v: "not-an-int-and-not-expr"`), &w)
	if err == nil {
		t.Error("expected error for invalid IntOrExpr value")
	}
}
