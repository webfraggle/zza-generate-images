package renderer

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func rotateTemplate(t *testing.T, rotate string) *Template {
	t.Helper()
	dir := t.TempDir()
	src := image.NewNRGBA(image.Rect(0, 0, 20, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 20; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	f, err := os.Create(filepath.Join(dir, "bar.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, src); err != nil {
		t.Fatal(err)
	}
	f.Close()
	tmpl := makeTemplate(60, 60, []Layer{{
		Type: "image", File: "bar.png",
		X: IntOrExpr{val: 30}, Y: IntOrExpr{val: 30},
		Rotate: StringOrCond{raw: rotate},
	}})
	tmpl.Dir = dir
	return tmpl
}

func TestRenderImage_RotateNonFinite(t *testing.T) {
	r := New("../../templates")
	for name, tc := range map[string]struct {
		rot  string
		data map[string]interface{}
	}{
		"NaN":      {"{{angle | mul(6)}}", map[string]interface{}{"angle": "NaN"}},
		"Inf":      {"{{angle}}", map[string]interface{}{"angle": "Inf"}},
		"overflow": {"{{angle | mul(10)}}", map[string]interface{}{"angle": "1e308"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := r.Render(rotateTemplate(t, tc.rot), tc.data); err == nil {
				t.Fatal("expected error for non-finite rotate")
			}
		})
	}
}

func TestRenderImage_RotateHugeAngleWraps(t *testing.T) {
	r := New("../../templates")
	a, err := r.Render(rotateTemplate(t, "36000090"), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Render(rotateTemplate(t, "90"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(a.Pix) != string(b.Pix) {
		t.Error("36000090 deg should render like 90 deg")
	}
}

func TestNumericArg_LiteralStrict(t *testing.T) {
	e := fixedEval(map[string]interface{}{"a": "5", "inf": "3", "nan": "2"})
	checkInterp(t, e, "{{a | add(inf)}}", "8")
	checkInterp(t, e, "{{a | add(nan)}}", "7")
	e2 := fixedEval(map[string]interface{}{"a": "5"})
	checkInterp(t, e2, "{{a | add(inf)}}", "")
	checkInterp(t, e2, "{{a | add(0x1p3)}}", "")
	checkInterp(t, e2, "{{a | add(2)}}", "7")
	checkInterp(t, e2, "{{a | add(-.5)}}", "4.5")
}

func TestMod_NeverEqualsDivisor(t *testing.T) {
	e := fixedEval(map[string]interface{}{"x": "-1e-20"})
	checkInterp(t, e, "{{x | mod(1440)}}", "0")
}


func TestPipeline_GoJSParityCases(t *testing.T) {
	d := pipeData()
	d["x"] = "-1e-20"
	e := fixedEval(d)
	checkInterp(t, e, "{{z1 | add(5)}}", "10:15")
	checkInterp(t, e, "{{empty | minutes | mod(1440)}}", "")
	checkInterp(t, e, "{{a | div(0)}}", "0")
	checkInterp(t, e, "{{a | mod(0)}}", "0")
	checkInterp(t, e, "{{a | add('x')}}", "5")
	checkInterp(t, e, "{{a | add(txt)}}", "")
	checkInterp(t, e, "{{a | add}}", "5")
	checkInterp(t, e, "{{a | add()}}", "5")
	checkInterp(t, e, "{{b | mul(3)}}", "7.5")
	checkInterp(t, e, "{{a | div(3)}}", "1.6666666666666667")
	checkInterp(t, e, "{{a | add(inf)}}", "")
	checkInterp(t, e, "{{a | add(a | add(a | add(a | add(a))))}}", "25")
	checkInterp(t, e, "{{a | add(a | add(a | add(a | add(a | add(a)))))}}", "")
	checkInterp(t, e, "{{x | mod(1440)}}", "0")
}
