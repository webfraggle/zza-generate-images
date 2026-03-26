// Package admin implements TOTP validation and admin session management.
package admin

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // SHA-1 is required by RFC 6238 TOTP
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ValidateTOTP checks whether code (6-digit string) matches the current or
// adjacent TOTP window for the given Base32-encoded secret.
// ±30 s tolerance is applied to accommodate clock skew.
// guard prevents replay attacks; pass nil to skip replay protection (tests only).
func ValidateTOTP(secretBase32, code string, guard *TOTPReplayGuard) bool {
	if len(code) != 6 {
		return false
	}
	codeInt, err := strconv.Atoi(code)
	if err != nil {
		return false
	}
	// Normalize: uppercase, remove spaces, strip optional padding.
	// TrimRight handles secrets from apps that include '=' padding.
	normalized := strings.TrimRight(strings.ToUpper(strings.ReplaceAll(secretBase32, " ", "")), "=")
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(normalized)
	if err != nil {
		return false
	}
	t := time.Now().Unix() / 30
	for _, counter := range []int64{t - 1, t, t + 1} {
		if totpCode(key, counter) == codeInt {
			if guard != nil && !guard.CheckAndMark(secretBase32, code, counter) {
				return false // replay
			}
			return true
		}
	}
	return false
}

func totpCode(key []byte, counter int64) int {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(counter))
	mac := hmac.New(sha1.New, key) //nolint:gosec // SHA-1 required by RFC 6238
	mac.Write(buf)
	h := mac.Sum(nil)
	offset := h[len(h)-1] & 0x0f
	code := int(h[offset]&0x7f)<<24 |
		int(h[offset+1]&0xff)<<16 |
		int(h[offset+2]&0xff)<<8 |
		int(h[offset+3]&0xff)
	return code % 1_000_000
}

// GenerateSecret returns a cryptographically random 20-byte Base32-encoded TOTP secret.
func GenerateSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("admin: generating TOTP secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// OTPAuthURL returns an otpauth:// URL for QR code generation.
func OTPAuthURL(secret, issuer, account string) string {
	return fmt.Sprintf(
		"otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		issuer, account, secret, issuer,
	)
}
