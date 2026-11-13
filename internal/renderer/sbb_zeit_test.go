package renderer

import (
	"bytes"
	"image/png"
	"testing"
)

// The PHP shows only hour:minute of zeit; without ":" the raw value.
func TestSbb105_ZeitSeconds(t *testing.T) {
	r, tmpl, data := loadSbb(t)
	render := func(zeit string) []byte {
		z := map[string]interface{}{}
		for k, v := range data["zug1"].(map[string]interface{}) {
			z[k] = v
		}
		z["zeit"] = zeit
		d := map[string]interface{}{"zug1": z}
		img, err := r.Render(tmpl, d)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}
	if !bytes.Equal(render("15:54:30"), render("15:54")) {
		t.Error("15:54:30 renders differently from 15:54")
	}
	if bytes.Equal(render("sofort"), render("")) {
		t.Error("zeit without ':' not shown")
	}
}
