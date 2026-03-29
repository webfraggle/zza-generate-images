package server

import (
	"sync"
	"time"
)

const (
	maxIPFailures   = 6
	ipBlockDuration = 6 * time.Hour
)

type ipEntry struct {
	failures     int
	blockedUntil time.Time
}

// IPLimiter tracks failed email attempts per client IP.
// After maxIPFailures failures, the IP is blocked for ipBlockDuration.
// State is in-memory and does not survive server restarts.
type IPLimiter struct {
	mu      sync.Mutex
	entries map[string]*ipEntry
}

func NewIPLimiter() *IPLimiter {
	return &IPLimiter{entries: make(map[string]*ipEntry)}
}

// Allow returns false if the IP is currently blocked.
func (l *IPLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[ip]
	if !ok {
		return true
	}
	if !e.blockedUntil.IsZero() && time.Now().Before(e.blockedUntil) {
		return false
	}
	return true
}

// RecordFailure increments the failure count for ip.
// Once failures reach maxIPFailures, the IP is blocked for ipBlockDuration.
func (l *IPLimiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[ip]
	if !ok {
		e = &ipEntry{}
		l.entries[ip] = e
	}
	if !e.blockedUntil.IsZero() {
		if time.Now().Before(e.blockedUntil) {
			// Still blocked — don't count additional failures.
			return
		}
		// Block has expired — reset counter and start fresh.
		e.failures = 0
		e.blockedUntil = time.Time{}
	}
	e.failures++
	if e.failures >= maxIPFailures {
		e.blockedUntil = time.Now().Add(ipBlockDuration)
	}
}

// RecordSuccess clears the failure record for ip.
func (l *IPLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}
