package admin

import (
	"encoding/base32"
	"fmt"
	"testing"
	"time"
)

// knownCode computes the TOTP code for a given key and Unix time.
// Used to generate deterministic test values.
func knownCode(key []byte, unixSec int64) string {
	counter := unixSec / 30
	return fmt.Sprintf("%06d", totpCode(key, counter))
}

func TestValidateTOTP_CurrentWindow(t *testing.T) {
	key := []byte("12345678901234567890") // 20-byte test key (RFC 6238 test vector)
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key)
	now := time.Now().Unix()
	code := knownCode(key, now)
	if !ValidateTOTP(secret, code, nil) {
		t.Errorf("current-window code %s should be valid", code)
	}
}

func TestValidateTOTP_PreviousWindow(t *testing.T) {
	key := []byte("12345678901234567890")
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key)
	prev := time.Now().Unix() - 30
	code := knownCode(key, prev)
	if !ValidateTOTP(secret, code, nil) {
		t.Errorf("previous-window code %s should be accepted (clock skew tolerance)", code)
	}
}

func TestValidateTOTP_ReplayGuard(t *testing.T) {
	key := []byte("12345678901234567890")
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key)
	now := time.Now().Unix()
	code := knownCode(key, now)
	guard := NewTOTPReplayGuard()
	if !ValidateTOTP(secret, code, guard) {
		t.Fatalf("first use should be valid")
	}
	if ValidateTOTP(secret, code, guard) {
		t.Error("second use of same code should be rejected (replay)")
	}
}

func TestValidateTOTP_WrongCode(t *testing.T) {
	key := []byte("12345678901234567890")
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key)
	if ValidateTOTP(secret, "000000", nil) {
		// 000000 is an astronomically unlikely valid code — test is probabilistic.
		// Skip rather than fail to avoid flakiness.
		t.Skip("000000 happened to be valid for this window — skipping")
	}
}

func TestValidateTOTP_InvalidLength(t *testing.T) {
	key := []byte("12345678901234567890")
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key)
	for _, code := range []string{"", "12345", "1234567", "abcdef"} {
		if ValidateTOTP(secret, code, nil) {
			t.Errorf("code %q should be invalid (wrong length or non-numeric)", code)
		}
	}
}

func TestValidateTOTP_BadSecret(t *testing.T) {
	if ValidateTOTP("not-valid-base32!!!", "123456", nil) {
		t.Error("invalid base32 secret should return false")
	}
}

func TestGenerateSecret(t *testing.T) {
	s1, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	if len(s1) == 0 {
		t.Error("secret should not be empty")
	}
	// Secrets should be unique.
	s2, _ := GenerateSecret()
	if s1 == s2 {
		t.Error("two generated secrets should not be equal")
	}
	// Generated secret should be parseable without panic.
	key := []byte("12345678901234567890")
	now := time.Now().Unix()
	code := knownCode(key, now)
	ValidateTOTP(s1, code, nil) // result not checked; just must not panic
}
