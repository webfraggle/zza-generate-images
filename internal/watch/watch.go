// Package watch observes the templates directory and publishes debounced
// per-template change batches. It is DESKTOP-ONLY (fsnotify must not end up in
// the server build); internal/editor only sees the editor.ChangeFeed interface.
package watch

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/webfraggle/zza-generate-images/internal/editor"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// ringSize is the number of batches kept for late pollers.
const ringSize = 256

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

type entry struct {
	seq      uint64
	template string
	files    []string
}

// Watcher implements editor.ChangeFeed on top of fsnotify.
type Watcher struct {
	root     string
	debounce time.Duration
	fsw      *fsnotify.Watcher

	mu      sync.Mutex
	seq     uint64
	ring    []entry
	notify  chan struct{} // closed and replaced on every publish
	pending map[string]map[string]struct{}
	timers  map[string]*time.Timer
	gens    map[string]uint64 // debounce generation per template
	// resetBefore: clients with since < resetBefore must reload everything
	// (set after an fsnotify event overflow).
	resetBefore uint64
	afterScan   func() // test hook: runs between ring scan and blocking
	callbacks   []func(template string, files []string)
}

var _ editor.ChangeFeed = (*Watcher)(nil)

// New starts watching templatesDir and its direct subdirectories.
func New(templatesDir string, debounce time.Duration) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("watch: %w", err)
	}
	root := filepath.Clean(templatesDir)
	if err := fsw.Add(root); err != nil {
		_ = fsw.Close()
		return nil, fmt.Errorf("watch: %w", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		_ = fsw.Close()
		return nil, fmt.Errorf("watch: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() && validName(e.Name()) {
			if err := fsw.Add(filepath.Join(root, e.Name())); err != nil {
				log.Printf("watch: add %q: %v", e.Name(), err)
			}
		}
	}
	return &Watcher{
		root: root, debounce: debounce, fsw: fsw,
		notify:  make(chan struct{}),
		pending: map[string]map[string]struct{}{},
		timers:  map[string]*time.Timer{},
		gens:    map[string]uint64{},
	}, nil
}

// OnChange registers a callback invoked for every published batch. Call it
// before Run.
func (w *Watcher) OnChange(fn func(template string, files []string)) {
	w.mu.Lock()
	w.callbacks = append(w.callbacks, fn)
	w.mu.Unlock()
}

func validName(n string) bool { return !strings.HasPrefix(n, ".") && nameRe.MatchString(n) }

// Run processes events until ctx is done.
func (w *Watcher) Run(ctx context.Context) {
	defer w.fsw.Close()
	for {
		select {
		case <-ctx.Done():
			w.mu.Lock()
			for _, t := range w.timers {
				t.Stop()
			}
			w.mu.Unlock()
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handle(ev)
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				w.overflow()
			}
			log.Printf("watch: %v", err)
		}
	}
}

func (w *Watcher) handle(ev fsnotify.Event) {
	if ev.Op == fsnotify.Chmod {
		return
	}
	base := filepath.Base(ev.Name)
	if !validName(base) || strings.HasSuffix(base, ".tmp") {
		return
	}
	dir := filepath.Dir(ev.Name)
	if dir == w.root {
		// Event on a template directory itself. Only structural changes count:
		// on Windows a Write is reported for the folder whenever a file inside
		// it is created/renamed/removed. Plain files in the root (README.md)
		// are not templates.
		if !ev.Has(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) ||
			renderer.ValidateTemplateName(base) != nil {
			return
		}
		if ev.Has(fsnotify.Create) {
			fi, err := os.Stat(ev.Name)
			if err != nil || !fi.IsDir() {
				return
			}
			if err := w.fsw.Add(ev.Name); err != nil {
				log.Printf("watch: add %q: %v", base, err)
			}
		}
		w.record(base, "*")
		return
	}
	if filepath.Dir(dir) != w.root {
		return
	}
	tpl := filepath.Base(dir)
	if !validName(tpl) {
		return
	}
	w.record(tpl, base)
}

func (w *Watcher) record(tpl, file string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	set := w.pending[tpl]
	if set == nil {
		set = map[string]struct{}{}
		w.pending[tpl] = set
	}
	set[file] = struct{}{}
	if t, ok := w.timers[tpl]; ok {
		t.Stop()
	}
	w.gens[tpl]++
	gen := w.gens[tpl]
	w.timers[tpl] = time.AfterFunc(w.debounce, func() { w.flush(tpl, gen) })
}

// overflow handles lost fsnotify events: every client must reload.
func (w *Watcher) overflow() {
	w.mu.Lock()
	w.seq++
	w.resetBefore = w.seq
	close(w.notify)
	w.notify = make(chan struct{})
	w.mu.Unlock()
}

func (w *Watcher) flush(tpl string, gen uint64) {
	w.mu.Lock()
	if w.gens[tpl] != gen {
		w.mu.Unlock()
		return
	}
	set := w.pending[tpl]
	delete(w.pending, tpl)
	delete(w.timers, tpl)
	if len(set) == 0 {
		w.mu.Unlock()
		return
	}
	files := make([]string, 0, len(set))
	for f := range set {
		files = append(files, f)
	}
	sort.Strings(files)
	w.seq++
	w.ring = append(w.ring, entry{seq: w.seq, template: tpl, files: files})
	if len(w.ring) > ringSize {
		w.ring = append([]entry(nil), w.ring[len(w.ring)-ringSize:]...)
	}
	close(w.notify)
	w.notify = make(chan struct{})
	cbs := make([]func(string, []string), len(w.callbacks))
	copy(cbs, w.callbacks)
	w.mu.Unlock()
	for _, cb := range cbs {
		cb(tpl, files)
	}
}

// Current returns the sequence number of the latest published batch.
func (w *Watcher) Current() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.seq
}

// Wait returns the merged changes for template with seq > since, waiting up to
// timeout. On timeout or cancelled ctx it returns {Seq: <seq of the scan>, Files: []}. If
// since is older than the ring (or newer than the current seq, e.g. after a
// restart) it returns Reset with Files ["*"]. A non-empty result carries the
// current seq, so unrelated templates' batches never push the client out of
// the ring.
func (w *Watcher) Wait(ctx context.Context, template string, since uint64, timeout time.Duration) editor.ChangeBatch {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		w.mu.Lock()
		cur := w.seq
		if since > cur || since < w.resetBefore || (len(w.ring) > 0 && since+1 < w.ring[0].seq) {
			w.mu.Unlock()
			return editor.ChangeBatch{Seq: cur, Files: []string{"*"}, Reset: true}
		}
		set := map[string]struct{}{}
		for _, e := range w.ring {
			if e.seq > since && e.template == template {
				for _, f := range e.files {
					set[f] = struct{}{}
				}
			}
		}
		ch := w.notify
		w.mu.Unlock()
		if len(set) > 0 {
			files := make([]string, 0, len(set))
			for f := range set {
				files = append(files, f)
			}
			sort.Strings(files)
			return editor.ChangeBatch{Seq: cur, Files: files}
		}
		if w.afterScan != nil {
			w.afterScan()
		}
		select {
		case <-ch:
		case <-timer.C:
			return editor.ChangeBatch{Seq: cur, Files: []string{}}
		case <-ctx.Done():
			return editor.ChangeBatch{Seq: cur, Files: []string{}}
		}
	}
}
