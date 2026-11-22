package renderer

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeTmpl writes a temp template and returns its templates dir and name.
func writeTmpl(t *testing.T, yaml string) (dir, name string) {
	t.Helper()
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "template.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(d), filepath.Base(d)
}

const frameFlapLayerYAML = `  - type: splitflap
    x: 0
    y: 0
    width: 10
    height: 10
    cells: 2
    split: chars
    value: "AB"
    reel: " AB"
    var: ch
    layers:
      - if: "eq(ch, 'A')"
        layers: [{type: rect, x: 0, y: 0, width: 10, height: 10, color: "#ff0000"}]
`

func TestAnimationMeta_Validate(t *testing.T) {
	cases := []struct {
		name, anim, extra, wantErr string
	}{
		{"frames+delay+loop", "frames: 60, frame_delay: 1, loop: true", "", ""},
		{"frames only", "frames: 60", "", ""},
		{"120 frames", "frames: 120, frame_delay: 1", "", ""},
		{"frames 1", "frames: 1", "", "between 2 and 125"},
		{"frames 126", "frames: 126", "", "between 2 and 125"},
		{"frames -1", "frames: -1", "", "between 2 and 125"},
		{"frames+fps", "frames: 60, fps: 20", "", "cannot be combined"},
		{"frames+duration", "frames: 60, duration: 2", "", "cannot be combined"},
		{"delay alone", "frame_delay: 1", "", "requires animation.frames"},
		{"delay too small", "frames: 10, frame_delay: 0.015", "", "between 0.02 and 10"},
		{"delay step", "frames: 10, frame_delay: 0.025", "", "steps of 0.01"},
		{"delay too big", "frames: 10, frame_delay: 10.5", "", "between 0.02 and 10"},
		{"too long", "frames: 125, frame_delay: 1", "", "must not exceed 120 seconds"},
		{"frames+splitflap", "frames: 2", frameFlapLayerYAML, "splitflap layers"},
		{"fps 60", "fps: 60", "", "animation.fps must be between"},
		{"loop only", "loop: true", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			y := "meta:\n  canvas: {width: 20, height: 10}\n  animation: {" + c.anim + "}\nlayers:\n" +
				"  - {type: rect, x: 0, y: 0, width: 10, height: 10, color: \"#000000\"}\n" + c.extra
			dir, name := writeTmpl(t, y)
			_, err := LoadTemplate(dir, name)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, c.wantErr)
			}
		})
	}

	dir, name := writeTmpl(t, "meta:\n  canvas: {width: 2, height: 2}\n  animation: {frames: 60}\nlayers:\n  - {type: rect, x: 0, y: 0, width: 1, height: 1, color: \"#000000\"}\n")
	tmpl, err := LoadTemplate(dir, name)
	if err != nil {
		t.Fatal(err)
	}
	if got := tmpl.Meta.Animation.frameDelayCS(); got != 100 {
		t.Errorf("frameDelayCS = %d, want 100", got)
	}
}

func TestIsAnimated(t *testing.T) {
	if !flapAnimTemplate(2, "AB").IsAnimated() {
		t.Error("splitflap template: IsAnimated = false")
	}
	f := makeTemplate(1, 1, nil)
	f.Meta.Animation.Frames = 60
	if !f.IsAnimated() {
		t.Error("frame template: IsAnimated = false")
	}
	if makeTemplate(1, 1, nil).IsAnimated() {
		t.Error("plain template: IsAnimated = true")
	}
}

func TestEvaluator_AnimVars(t *testing.T) {
	data := map[string]interface{}{"anim": map[string]interface{}{"frame": "x"}}
	e := NewEvaluator(data).withAnim(3, 60, 100)
	for in, want := range map[string]string{
		"{{anim.frame}}":  "3",
		"{{anim.frames}}": "60",
		"{{anim.time}}":   "3",
		"{{anim.foo}}":    "",
	} {
		if got := e.Interpolate(in); got != want {
			t.Errorf("%s = %q, want %q", in, got, want)
		}
	}
	if got := NewEvaluator(nil).withAnim(3, 10, 10).Interpolate("{{anim.time}}"); got != "0.3" {
		t.Errorf("anim.time = %q, want 0.3", got)
	}
	if v, err := evalIntExpr("anim.frame * 2 + anim.frames", e.intVars); err != nil || v != 66 {
		t.Errorf("evalIntExpr = %d, %v, want 66", v, err)
	}
	l := e.withLoopVars(map[string]string{"v": "a"}, map[string]int{"i": 1})
	if got := l.Interpolate("{{anim.frame}}"); got != "3" {
		t.Errorf("loop {{anim.frame}} = %q, want 3", got)
	}
	if v, err := evalIntExpr("anim.frame + i", l.intVars); err != nil || v != 4 {
		t.Errorf("loop evalIntExpr = %d, %v, want 4", v, err)
	}
	n := NewEvaluator(nil)
	if got := n.Interpolate("{{anim.frame}}"); got != "0" {
		t.Errorf("default anim.frame = %q, want 0", got)
	}
	if got := n.Interpolate("{{anim.frames}}"); got != "1" {
		t.Errorf("default anim.frames = %q, want 1", got)
	}
}

var (
	red   = color.NRGBA{255, 0, 0, 255}
	black = color.NRGBA{0, 0, 0, 255}
)

// frameModeTemplate is 10×1 with a black background and a red pixel at x = anim.frame.
func frameModeTemplate(frames int, delay float64, loop bool) *Template {
	dot := rectLayer(0, 0, 1, 1, "#ff0000")
	dot.X = IntOrExpr{expr: "anim.frame", isExpr: true}
	tmpl := makeTemplate(10, 1, []Layer{rectLayer(0, 0, 10, 1, "#000000"), dot})
	tmpl.Meta.Animation = AnimationMeta{Frames: frames, FrameDelay: delay, Loop: loop}
	return tmpl
}

func fixedRenderer() *Renderer {
	r := New("/tmp")
	r.nowFunc = func() time.Time { return fixedNow }
	return r
}

func TestRenderAnimation_FrameMode(t *testing.T) {
	a := mustAnimate(t, fixedRenderer(), frameModeTemplate(5, 0.5, true), nil, nil)
	if len(a.Frames) != 5 || a.Delay != 50 || !a.Loop {
		t.Fatalf("frames=%d delay=%d loop=%v, want 5/50/true", len(a.Frames), a.Delay, a.Loop)
	}
	for f, img := range a.Frames {
		for x := 0; x < 10; x++ {
			want := black
			if x == f {
				want = red
			}
			assertPixel(t, img, x, 0, want)
		}
	}
}

func TestRenderAnimation_FrameMode_StringVars(t *testing.T) {
	l := rectLayer(0, 0, 1, 1, "")
	l.Color = StringOrCond{cond: &condMap{
		branches: []condBranch{{ifExpr: "equals(anim.frame, '1')", then: "#ff0000"}},
		els:      "#0000ff",
	}}
	tmpl := makeTemplate(1, 1, []Layer{l})
	tmpl.Meta.Animation = AnimationMeta{Frames: 3}
	a := mustAnimate(t, fixedRenderer(), tmpl, nil, nil)
	blue := color.NRGBA{0, 0, 255, 255}
	for f, want := range []color.NRGBA{blue, red, blue} {
		assertPixel(t, a.Frames[f], 0, 0, want)
	}

	green := color.NRGBA{0, 255, 0, 255}
	tmpl.Layers = []Layer{l, blockIf("equals(anim.frames, '3')", []Layer{rectLayer(0, 0, 1, 1, "#00ff00")})}
	a = mustAnimate(t, fixedRenderer(), tmpl, nil, nil)
	for f := range a.Frames {
		assertPixel(t, a.Frames[f], 0, 0, green)
	}
}

func TestRenderAnimation_FrameMode_NowAdvancesFromOneClockRead(t *testing.T) {
	r := New("/tmp")
	reads := 0
	r.nowFunc = func() time.Time { reads++; return fixedNow } // 09:15:30
	l := rectLayer(0, 0, 1, 1, "")
	l.Color = StringOrCond{cond: &condMap{
		branches: []condBranch{{ifExpr: "equals(now.second, '30')", then: "#ff0000"}},
		els:      "#00ff00",
	}}
	tmpl := makeTemplate(1, 1, []Layer{l})
	tmpl.Meta.Animation = AnimationMeta{Frames: 4} // default delay 1 s
	green := color.NRGBA{0, 255, 0, 255}
	for f, img := range mustAnimate(t, r, tmpl, nil, nil).Frames {
		want := green
		if f == 0 {
			want = red
		}
		if got := img.NRGBAAt(0, 0); got != want {
			t.Errorf("frame %d = %v, want %v", f, got, want)
		}
	}
	if reads != 1 {
		t.Errorf("clock read %d times, want once", reads)
	}
}

func TestRenderAnimation_FrameMode_FirstFrameEqualsRender(t *testing.T) {
	r := fixedRenderer()
	tmpl := frameModeTemplate(5, 0.5, true)
	a := mustAnimate(t, r, tmpl, nil, nil)
	if static := mustRender(t, r, tmpl, nil); !bytes.Equal(static.Pix, a.Frames[0].Pix) {
		t.Error("Render differs from frame 0")
	}
	b := mustAnimate(t, r, tmpl, nil, nil)
	if !framesEqual(a.Frames, b.Frames) {
		t.Error("second RenderAnimation call differs")
	}
}

func TestRenderAnimation_FrameMode_PixelLimit(t *testing.T) {
	tmpl := makeTemplate(1000, 1000, []Layer{rectLayer(0, 0, 1, 1, "#000000")})
	tmpl.Meta.Animation = AnimationMeta{Frames: 40}
	_, err := fixedRenderer().RenderAnimation(tmpl, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "animation too large") {
		t.Fatalf("error = %v, want animation too large", err)
	}
}

func TestRender_StaticAnimVars(t *testing.T) {
	img := mustRender(t, fixedRenderer(), frameModeTemplate(5, 0.5, true), nil)
	assertPixel(t, img, 0, 0, red)
	assertPixel(t, img, 1, 0, black)
}

func TestRenderAnimation_SplitflapLoop(t *testing.T) {
	tmpl := flapAnimTemplate(4, "AB")
	tmpl.Meta.Animation.Loop = true
	a := mustAnimate(t, New("/tmp"), tmpl, nil, []byte("s"))
	if !a.Loop || a.Delay != 0 {
		t.Errorf("Loop=%v Delay=%d, want true/0", a.Loop, a.Delay)
	}
	tmpl.Meta.Animation.Loop = false
	if a := mustAnimate(t, New("/tmp"), tmpl, nil, []byte("s")); a.Loop {
		t.Error("Loop = true without loop")
	}
}
