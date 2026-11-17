package renderer

import "time"

// NewWithClock is New with an injected clock, so tests can render with a
// fixed `now` (the device core is compared against Go this way); the
// renderer itself does not use it.
func NewWithClock(dir string, now func() time.Time) *Renderer {
	r := New(dir)
	r.nowFunc = now
	return r
}
