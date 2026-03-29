package server

import (
	"testing"
	"time"
)

func TestIPLimiter_AllowBeforeLimit(t *testing.T) {
	l := NewIPLimiter()
	for i := 0; i < maxIPFailures-1; i++ {
		l.RecordFailure("1.2.3.4")
	}
	if !l.Allow("1.2.3.4") {
		t.Error("expected Allow=true after 5 failures")
	}
}

func TestIPLimiter_BlockedAtLimit(t *testing.T) {
	l := NewIPLimiter()
	for i := 0; i < maxIPFailures; i++ {
		l.RecordFailure("1.2.3.4")
	}
	if l.Allow("1.2.3.4") {
		t.Error("expected Allow=false after 6 failures")
	}
}

func TestIPLimiter_ExpiredBlock(t *testing.T) {
	l := NewIPLimiter()
	for i := 0; i < maxIPFailures; i++ {
		l.RecordFailure("1.2.3.4")
	}
	// Manually backdate the block so it appears expired.
	l.mu.Lock()
	l.entries["1.2.3.4"].blockedUntil = time.Now().Add(-time.Minute)
	l.mu.Unlock()
	if !l.Allow("1.2.3.4") {
		t.Error("expected Allow=true after block expired")
	}
}

func TestIPLimiter_RecordSuccessResets(t *testing.T) {
	l := NewIPLimiter()
	for i := 0; i < maxIPFailures; i++ {
		l.RecordFailure("1.2.3.4")
	}
	l.RecordSuccess("1.2.3.4")
	if !l.Allow("1.2.3.4") {
		t.Error("expected Allow=true after RecordSuccess")
	}
}

func TestIPLimiter_CounterResetsAfterExpiry(t *testing.T) {
	l := NewIPLimiter()
	// Get blocked.
	for i := 0; i < maxIPFailures; i++ {
		l.RecordFailure("1.2.3.4")
	}
	// Expire the block.
	l.mu.Lock()
	l.entries["1.2.3.4"].blockedUntil = time.Now().Add(-time.Minute)
	l.mu.Unlock()
	// One failure after expiry should NOT re-block immediately.
	l.RecordFailure("1.2.3.4")
	if !l.Allow("1.2.3.4") {
		t.Error("expected Allow=true: single failure after expiry should not re-block")
	}
}
