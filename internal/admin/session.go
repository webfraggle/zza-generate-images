package admin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const (
	// AdminCookieName is the name of the admin session cookie.
	AdminCookieName = "zza_admin"
	// SessionTTL is how long an admin session remains valid.
	SessionTTL = 8 * time.Hour

	maxLoginAttempts = 5
	loginWindow      = 15 * time.Minute
)

// SessionStore holds short-lived admin session tokens in memory.
// There is no persistence — sessions are lost on server restart.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]time.Time // token → expiry
}

// NewSessionStore returns an empty, ready-to-use session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]time.Time)}
}

// Create generates a new 64-char hex session token, stores it and returns it.
func (ss *SessionStore) Create() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b)
	ss.mu.Lock()
	ss.sessions[tok] = time.Now().Add(SessionTTL)
	ss.mu.Unlock()
	return tok, nil
}

// Validate returns true if tok is a valid, non-expired session token.
// Expired tokens are evicted on first access.
func (ss *SessionStore) Validate(tok string) bool {
	if tok == "" {
		return false
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	exp, ok := ss.sessions[tok]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(ss.sessions, tok)
		return false
	}
	return true
}

// Destroy removes a session token immediately.
func (ss *SessionStore) Destroy(tok string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.sessions, tok)
}

// LoginLimiter tracks per-IP login attempts to prevent brute-force attacks.
// IPs are taken from net.SplitHostPort(r.RemoteAddr) — never from headers.
type LoginLimiter struct {
	mu     sync.Mutex
	counts map[string][]time.Time // IP → attempt timestamps within loginWindow
}

// NewLoginLimiter returns an empty login limiter.
func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{counts: make(map[string][]time.Time)}
}

// Allow returns true if ip has not exceeded maxLoginAttempts within loginWindow.
// Each call that returns true counts as one attempt.
func (l *LoginLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-loginWindow)
	var recent []time.Time
	for _, t := range l.counts[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= maxLoginAttempts {
		l.counts[ip] = recent
		return false
	}
	l.counts[ip] = append(recent, now)
	return true
}

// TOTPReplayGuard prevents re-use of TOTP codes within their validity window.
// A code is valid for at most 3 windows (±30 s), so entries expire after 90 s.
type TOTPReplayGuard struct {
	mu   sync.Mutex
	used map[string]time.Time // "secretFingerprint:code:counter" → first-seen time
}

// NewTOTPReplayGuard returns an empty guard.
func NewTOTPReplayGuard() *TOTPReplayGuard {
	return &TOTPReplayGuard{used: make(map[string]time.Time)}
}

// CheckAndMark returns true (and marks the code as used) if the combination
// of secretFingerprint+code+counter has not been seen before.
// Returns false on replay.
func (g *TOTPReplayGuard) CheckAndMark(secretFingerprint, code string, counter int64) bool {
	key := fmt.Sprintf("%s:%s:%d", secretFingerprint, code, counter)
	g.mu.Lock()
	defer g.mu.Unlock()
	// Evict expired entries (>90 s old).
	now := time.Now()
	for k, t := range g.used {
		if now.Sub(t) > 90*time.Second {
			delete(g.used, k)
		}
	}
	if _, seen := g.used[key]; seen {
		return false
	}
	g.used[key] = now
	return true
}
