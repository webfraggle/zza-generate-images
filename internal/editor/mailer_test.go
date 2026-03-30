package editor

import (
	"testing"
	"time"
)

func TestSendTokenMail_HeaderInjection(t *testing.T) {
	cfg := MailConfig{} // Host empty — would skip actual sending
	ttl := time.Hour

	cases := []struct {
		name         string
		to           string
		templateName string
	}{
		{"CR in to", "victim@example.com\rBcc: attacker@evil.com", "my-template"},
		{"LF in to", "victim@example.com\nBcc: attacker@evil.com", "my-template"},
		{"CR in templateName", "victim@example.com", "my-template\rBcc: attacker@evil.com"},
		{"LF in templateName", "victim@example.com", "my-template\nBcc: attacker@evil.com"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := SendTokenMail(cfg, tc.to, tc.templateName, "sometoken", ttl)
			if err == nil {
				t.Error("expected error for header injection attempt, got nil")
			}
		})
	}
}
