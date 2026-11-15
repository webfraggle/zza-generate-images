package watch

import (
	"context"
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
	if len(b.Files) != 0 || b.Seq != since {
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
		w.flush("a-105-v1")
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
