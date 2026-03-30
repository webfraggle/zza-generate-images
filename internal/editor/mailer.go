package editor

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

// MailConfig holds SMTP connection settings.
type MailConfig struct {
	Host    string
	Port    string
	User    string
	Pass    string
	From    string
	BaseURL string
}

// SendTokenMail sends the edit link to the given address.
// auth is skipped when User is empty (anonymous SMTP relay).
func SendTokenMail(cfg MailConfig, to, templateName, token string, ttl time.Duration) error {
	for _, s := range []string{to, templateName} {
		if strings.ContainsAny(s, "\r\n") {
			return fmt.Errorf("mailer: header injection attempt in %q", s)
		}
	}

	editURL := strings.TrimRight(cfg.BaseURL, "/") + "/edit/" + token

	hours := int(ttl.Hours())
	ttlStr := fmt.Sprintf("%d Stunden", hours)
	if hours == 1 {
		ttlStr = "1 Stunde"
	}

	subject := fmt.Sprintf("Editier-Link für Template »%s«", templateName)
	body := fmt.Sprintf(
		"Hallo,\r\n\r\n"+
			"hier ist dein Editier-Link für das Template »%s«:\r\n\r\n"+
			"  %s\r\n\r\n"+
			"Der Link ist für %s gültig.\r\n\r\n"+
			"Falls du keinen Editier-Link angefordert hast, kannst du diese E-Mail ignorieren.\r\n\r\n"+
			"-- Zugzielanzeiger\r\n",
		templateName, editURL, ttlStr,
	)

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		cfg.From, to, subject, body,
	)

	addr := cfg.Host + ":" + cfg.Port
	var auth smtp.Auth
	if cfg.User != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)
	}

	if err := smtp.SendMail(addr, auth, cfg.From, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("mailer: sending to %q: %w", to, err)
	}
	return nil
}
