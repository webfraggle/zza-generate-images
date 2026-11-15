package editor

import (
	"errors"
	"testing"
)

func TestValidateWriteName(t *testing.T) {
	for _, ok := range []string{"template.yaml", "default.json", "bg.png", "console.json", "con1.png", "com0.png", "lpt.json", "a.b.json", "nullable.json"} {
		if err := ValidateWriteName(ok); err != nil {
			t.Errorf("%q: unexpected %v", ok, err)
		}
	}
	for _, bad := range []string{"", ".json", ".hidden.png", "CON.json", "nul.yaml", "con", "Aux.PNG", "prn.tar.json", "COM1.png", "lpt9.json", "a/b.json", "x y.json", "..", "."} {
		if err := ValidateWriteName(bad); !errors.Is(err, ErrInvalidName) {
			t.Errorf("%q: got %v, want ErrInvalidName", bad, err)
		}
	}
}

func TestWriteTextFile_ReservedNameRejected(t *testing.T) {
	dir, name := makeTestTemplate(t)
	for _, n := range []string{"CON.json", ".json", "nul.yaml"} {
		if err := WriteTextFile(dir, name, n, []byte("{}")); !errors.Is(err, ErrInvalidName) {
			t.Errorf("%q: got %v", n, err)
		}
	}
}
