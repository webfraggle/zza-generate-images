package editor

import (
	"context"
	"time"
)

// ChangeBatch is one debounced set of file changes of a template.
type ChangeBatch struct {
	Seq   uint64   `json:"seq"`
	Files []string `json:"files"`
	Reset bool     `json:"reset,omitempty"`
}

// ChangeFeed is the source of change events for the long-poll route. It is
// implemented by internal/watch (desktop only).
type ChangeFeed interface {
	Wait(ctx context.Context, template string, since uint64, timeout time.Duration) ChangeBatch
	Current() uint64
}
