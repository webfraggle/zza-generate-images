package renderer

import "testing"

func TestUsesNow(t *testing.T) {
	val := func(s string) Layer { return Layer{Type: "text", Value: StringOrCond{raw: s}} }
	cases := []struct {
		name  string
		layer Layer
		want  bool
	}{
		{"now.hour", val("{{now.hour}}"), true},
		{"format filter", val("{{zug1.zeit | format('HH:mm')}}"), true},
		{"format with offset", val("{{now | format('HH:mm:ss', anim.time)}}"), true},
		{"now as filter arg", val("{{x | add(now.second)}}"), true},
		{"if now", Layer{Type: "rect", If: "greaterThan(now.hour, 5)"}, true},
		{"elif now", Layer{Type: "rect", Elif: "greaterThan(now.hour, 5)"}, true},
		{"color then", Layer{Type: "rect", Color: StringOrCond{cond: &condMap{
			branches: []condBranch{{ifExpr: "equals(a, 'b')", then: "{{now.second}}"}}, els: "#000000"}}}, true},
		{"color else", Layer{Type: "rect", Color: StringOrCond{cond: &condMap{
			branches: []condBranch{{ifExpr: "equals(a, 'b')", then: "#ffffff"}}, els: "{{now.second}}"}}}, true},
		{"cond ifExpr", Layer{Type: "rect", Color: StringOrCond{cond: &condMap{
			branches: []condBranch{{ifExpr: "equals(now.minute, '0')", then: "#ffffff"}}, els: "#000000"}}}, true},
		{"rotate", Layer{Type: "image", Rotate: StringOrCond{raw: "{{now.minute | mul(6)}}"}}, true},
		{"nested block", Layer{Layers: []Layer{val("{{now.hour}}")}}, true},
		{"nested loop", Layer{Type: "loop", Layers: []Layer{val("{{now.hour}}")}}, true},
		{"nested splitflap", Layer{Type: "splitflap", Layers: []Layer{val("{{now.hour}}")}}, true},
		{"literal now", val("now"), false},
		{"unknown text", val("Unknown {{zug1.nr}}"), false},
		{"nowhere", val("{{nowhere}}"), false},
		{"snow.x", val("{{snow.x}}"), false},
		{"zug1.now", val("{{zug1.now}}"), false},
		{"anim.frame", val("{{anim.frame}}"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpl := makeTemplate(1, 1, []Layer{c.layer})
			if got := tmpl.UsesNow(); got != c.want {
				t.Errorf("UsesNow() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestUsesNow_RealTemplates(t *testing.T) {
	want := map[string]bool{
		"nederland-105-v1": true, "oebb-105-v1": true, "umuc-105-v1": true,
		"faltblatt-105-v1": false, "rhb-105-v1": false,
		"sbb-105-v1": false,
	}
	for name, w := range want {
		tmpl, err := LoadTemplate("../../templates", name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got := tmpl.UsesNow(); got != w {
			t.Errorf("%s: UsesNow() = %v, want %v", name, got, w)
		}
	}
}

func TestClockGranularity(t *testing.T) {
	val := func(s string) Layer { return Layer{Type: "text", Value: StringOrCond{raw: s}} }
	cases := []struct {
		name  string
		layer Layer
		want  string
	}{
		{"no now", val("Hallo {{zug1.nr}}"), "none"},
		{"now", val("{{now}}"), "minute"},
		{"now.day", val("{{now.day}}"), "minute"},
		{"format date", val("{{now | format('dd.MM.')}}"), "minute"},
		{"now.second", val("{{now.second | mul(6)}}"), "second"},
		{"format with seconds", val("{{now | format('HH:mm:ss')}}"), "second"},
		{"format non-literal pattern", val("{{now | format(muster)}}"), "second"},
		{"format offset not whole minutes", val("{{now | format('HH:mm', 90)}}"), "second"},
		{"format offset whole minutes", val("{{now | format('HH:mm', 120)}}"), "minute"},
		{"format negative whole minutes", val("{{now | format('HH:mm', -3600)}}"), "minute"},
		{"format offset variable", val("{{now | format('HH:mm', anim.time)}}"), "second"},
		{"format offset pipeline", val("{{now | format('HH:mm', zug1.vl | mul(60))}}"), "second"},
		{"format offset fraction", val("{{now | format('HH:mm', 60.5)}}"), "second"},
		{"now.second in nested if", Layer{Layers: []Layer{{Type: "rect", If: "greaterThan(now.second, 30)"}}}, "second"},
		{"now.second outside braces", val("now.second"), "none"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpl := makeTemplate(1, 1, []Layer{c.layer})
			if got := tmpl.ClockGranularity(); got != c.want {
				t.Errorf("ClockGranularity() = %q, want %q", got, c.want)
			}
		})
	}
}
