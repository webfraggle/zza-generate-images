package watch

import (
	"context"
	"github.com/fsnotify/fsnotify"
	"github.com/webfraggle/zza-generate-images/internal/editor"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func setup(t *testing.T) (*Watcher, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a-105-v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	w, err := New(dir, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go w.Run(ctx)
	t.Cleanup(cancel)
	return w, dir
}

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWatcher_ReportsFileChange(t *testing.T) {
	w, dir := setup(t)
	write(t, filepath.Join(dir, "a-105-v1", "template.yaml"), "x")
	b := w.Wait(context.Background(), "a-105-v1", 0, 2*time.Second)
	if !reflect.DeepEqual(b.Files, []string{"template.yaml"}) || b.Seq < 1 {
		t.Fatalf("got %+v", b)
	}
}

func TestWatcher_Debounce(t *testing.T) {
	w, dir := setup(t)
	for i := 0; i < 5; i++ {
		write(t, filepath.Join(dir, "a-105-v1", "template.yaml"), "x")
		write(t, filepath.Join(dir, "a-105-v1", "default.json"), "x")
	}
	b := w.Wait(context.Background(), "a-105-v1", 0, 2*time.Second)
	if !reflect.DeepEqual(b.Files, []string{"default.json", "template.yaml"}) {
		t.Fatalf("got %+v", b)
	}
	time.Sleep(200 * time.Millisecond)
	if w.Current() != 1 {
		t.Fatalf("want exactly one batch, seq=%d", w.Current())
	}
}

func TestWatcher_IgnoresTmpHiddenAndChmodOnly(t *testing.T) {
	w, dir := setup(t)
	td := filepath.Join(dir, "a-105-v1")
	write(t, filepath.Join(td, "template.yaml"), "x")
	b := w.Wait(context.Background(), "a-105-v1", 0, 2*time.Second)
	since := b.Seq
	write(t, filepath.Join(td, "x.yaml.tmp"), "x")
	write(t, filepath.Join(td, ".DS_Store"), "x")
	now := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(td, "template.yaml"), now, now); err != nil {
		t.Fatal(err)
	}
	b = w.Wait(context.Background(), "a-105-v1", since, 500*time.Millisecond)
	if len(b.Files) != 0 || b.Seq != since { // no new batch: current == since
		t.Fatalf("got %+v", b)
	}
}

func TestWatcher_NewTemplateDir(t *testing.T) {
	w, dir := setup(t)
	if err := os.Mkdir(filepath.Join(dir, "b-105-v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := w.Wait(context.Background(), "b-105-v1", 0, 2*time.Second)
	if !reflect.DeepEqual(b.Files, []string{"*"}) {
		t.Fatalf("got %+v", b)
	}
	write(t, filepath.Join(dir, "b-105-v1", "template.yaml"), "x")
	b = w.Wait(context.Background(), "b-105-v1", b.Seq, 2*time.Second)
	if !reflect.DeepEqual(b.Files, []string{"template.yaml"}) {
		t.Fatalf("got %+v", b)
	}
}

func TestWatcher_OtherTemplateNotReported(t *testing.T) {
	w, dir := setup(t)
	write(t, filepath.Join(dir, "a-105-v1", "template.yaml"), "x")
	b := w.Wait(context.Background(), "c-105-v1", 0, 400*time.Millisecond)
	if len(b.Files) != 0 {
		t.Fatalf("got %+v", b)
	}
}

func TestWatcher_SinceAndReset(t *testing.T) {
	w, _ := setup(t)
	for i := 0; i < 300; i++ {
		w.record("a-105-v1", "template.yaml")
		w.mu.Lock()
		gen := w.gens["a-105-v1"]
		w.mu.Unlock()
		w.flush("a-105-v1", gen)
	}
	b := w.Wait(context.Background(), "a-105-v1", 0, time.Second)
	if !b.Reset || !reflect.DeepEqual(b.Files, []string{"*"}) {
		t.Fatalf("want reset, got %+v", b)
	}
	cur := w.Current()
	b = w.Wait(context.Background(), "a-105-v1", cur, 200*time.Millisecond)
	if b.Reset || len(b.Files) != 0 || b.Seq != cur {
		t.Fatalf("want timeout, got %+v", b)
	}
}

func TestWatcher_OnChangeCallback(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "a-105-v1"), 0o755)
	w, err := New(dir, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	type call struct {
		t string
		f []string
	}
	ch := make(chan call, 4)
	w.OnChange(func(tpl string, f []string) { ch <- call{tpl, f} })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)
	write(t, filepath.Join(dir, "a-105-v1", "template.yaml"), "x")
	select {
	case c := <-ch:
		if c.t != "a-105-v1" || !reflect.DeepEqual(c.f, []string{"template.yaml"}) {
			t.Fatalf("got %+v", c)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no callback")
	}
}

func TestWatcher_ContextCancel(t *testing.T) {
	w, _ := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	b := w.Wait(ctx, "a-105-v1", 0, 5*time.Second)
	if len(b.Files) != 0 || time.Since(start) > time.Second {
		t.Fatalf("got %+v after %v", b, time.Since(start))
	}
}

func TestWatcher_AtomicSaveReportsOnlyFile(t *testing.T) {
	w, dir := setup(t)
	if err := editor.WriteTextFile(dir, "a-105-v1", "template.yaml", []byte("x: 1\n")); err != nil {
		t.Fatal(err)
	}
	b := w.Wait(context.Background(), "a-105-v1", 0, 2*time.Second)
	if !reflect.DeepEqual(b.Files, []string{"template.yaml"}) {
		t.Fatalf("got %+v", b)
	}
}

func TestWatcher_RootWriteEventIgnored(t *testing.T) {
	w, dir := setup(t)
	// Windows reports Write on the folder itself; README.md is no template.
	w.handle(fsnotify.Event{Name: filepath.Join(dir, "a-105-v1"), Op: fsnotify.Write})
	w.handle(fsnotify.Event{Name: filepath.Join(dir, "README.md"), Op: fsnotify.Create})
	w.handle(fsnotify.Event{Name: filepath.Join(dir, "README.md"), Op: fsnotify.Remove})
	b := w.Wait(context.Background(), "a-105-v1", 0, 300*time.Millisecond)
	if len(b.Files) != 0 || w.Current() != 0 {
		t.Fatalf("got %+v seq=%d", b, w.Current())
	}
}

func TestWatcher_OverflowResets(t *testing.T) {
	w, _ := setup(t)
	w.overflow()
	b := w.Wait(context.Background(), "a-105-v1", 0, time.Second)
	if !b.Reset || b.Seq != 1 {
		t.Fatalf("want reset, got %+v", b)
	}
	b = w.Wait(context.Background(), "a-105-v1", b.Seq, 100*time.Millisecond)
	if b.Reset || len(b.Files) != 0 {
		t.Fatalf("reset must not repeat: %+v", b)
	}
}

func TestWatcher_StaleDebounceIgnored(t *testing.T) {
	w, _ := setup(t)
	w.record("a-105-v1", "x")
	w.flush("a-105-v1", 0) // stale generation
	if w.Current() != 0 {
		t.Fatal("stale flush published")
	}
}
