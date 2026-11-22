package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTemplatesDir_FlagOverrideWins(t *testing.T) {
	override := t.TempDir()
	got, err := ResolveTemplatesDir(override, "/unused/exe")
	if err != nil {
		t.Fatal(err)
	}
	if got != override {
		t.Errorf("got %q, want %q", got, override)
	}
}

func TestResolveTemplatesDir_BareBinaryUsesExeDir(t *testing.T) {
	tmp := t.TempDir()
	fakeExe := filepath.Join(tmp, "zza")
	if err := os.WriteFile(fakeExe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveTemplatesDir("", fakeExe)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(tmp, "templates")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveTemplatesDir_AppBundleUsesSiblingDir(t *testing.T) {
	tmp := t.TempDir()
	bundle := filepath.Join(tmp, "ZZA.app", "Contents", "MacOS", "zza")
	if err := os.MkdirAll(filepath.Dir(bundle), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bundle, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveTemplatesDir("", bundle)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(tmp, "templates") // sibling of ZZA.app
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEnsureTemplatesDir_CreatesAndSeeds(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "templates")
	if err := EnsureTemplatesDir(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "default", "template.yaml")); err != nil {
		t.Errorf("starter template not created: %v", err)
	}
}
