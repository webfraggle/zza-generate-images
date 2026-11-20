package devicerender

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const syncScript = "../../tools/sync-zzarender.sh"

// TestSyncScript: sync into a temp firmware dir, --check detects changed and
// extra files, a new sync removes stale files again; a directory without
// platformio.ini is refused untouched. Never touches a real firmware repo.
func TestSyncScript(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash")
	}
	// fwDir: a temp dir that looks like the firmware repo (platformio.ini)
	fwDir := func() string {
		d := t.TempDir()
		if err := os.WriteFile(filepath.Join(d, "platformio.ini"), []byte("[env]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return d
	}
	fw := fwDir()
	run := func(args ...string) (string, int) {
		out, err := exec.Command(bash, append([]string{syncScript}, args...)...).CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return string(out), code
	}
	if out, code := run(fw); code != 0 {
		t.Fatalf("sync: %d\n%s", code, out)
	}
	dst := filepath.Join(fw, "lib", "zzarender")
	for _, f := range []string{"eval.cpp", "zzarender_c.h", "third_party/ArduinoJson.h", "SOURCE.txt"} {
		if _, err := os.Stat(filepath.Join(dst, f)); err != nil {
			t.Errorf("%s not copied: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dst, "tools")); err == nil {
		t.Error("tools/ was copied")
	}

	src, err := os.ReadFile(filepath.Join(dst, "SOURCE.txt"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(src)), "\n")
	if len(lines) < 4 || !strings.HasPrefix(lines[0], "commit ") || !strings.HasPrefix(lines[1], "date ") {
		t.Fatalf("SOURCE.txt header: %q", lines[:min(2, len(lines))])
	}
	for i := 2; i < len(lines); i++ {
		if len(lines[i]) < 68 || lines[i][64:66] != "  " {
			t.Fatalf("SOURCE.txt line %d: %q", i+1, lines[i])
		}
		if i > 2 && lines[i][66:] <= lines[i-1][66:] {
			t.Errorf("SOURCE.txt not sorted at %q", lines[i])
		}
	}

	if out, code := run("--check", fw); code != 0 {
		t.Fatalf("--check right after the sync: %d\n%s", code, out)
	}

	// A changed file is reported by name.
	changed := filepath.Join(dst, "eval.cpp")
	orig, err := os.ReadFile(changed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(changed, append(orig, []byte("// edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := run("--check", fw); code != 1 || !strings.Contains(out, "eval.cpp") {
		t.Fatalf("--check with a changed file: %d\n%s", code, out)
	}

	// An extra file is reported and removed by the next sync (which also restores eval.cpp).
	extra := filepath.Join(dst, "stale.cpp")
	if err := os.WriteFile(extra, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := run("--check", fw); code != 1 || !strings.Contains(out, "stale.cpp") {
		t.Fatalf("--check with an extra file: %d\n%s", code, out)
	}
	if out, code := run(fw); code != 0 {
		t.Fatalf("second sync: %d\n%s", code, out)
	}
	if _, err := os.Stat(extra); err == nil {
		t.Error("stale file still there after the sync")
	}
	if out, code := run("--check", fw); code != 0 {
		t.Fatalf("--check after the second sync: %d\n%s", code, out)
	}
	if code := func() int { _, c := run("--check", fwDir()); return c }(); code != 1 {
		t.Errorf("--check on an empty firmware dir: exit %d, want 1", code)
	}

	// Not a firmware repo (no platformio.ini): refused with exit 2, nothing written or removed.
	other := t.TempDir()
	keep := filepath.Join(other, "lib", "zzarender", "keep.txt")
	if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{other}, {"--check", other}} {
		if out, code := run(args...); code != 2 || !strings.Contains(out, "platformio.ini") {
			t.Errorf("%v on a non-firmware dir: exit %d, want 2\n%s", args, code, out)
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("sync touched a non-firmware dir: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(other, "lib")); len(entries) != 1 {
		t.Errorf("sync left files in a non-firmware dir: %v", entries)
	}
}
