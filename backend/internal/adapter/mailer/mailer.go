// Package mailer sends email over SMTP (Mailpit locally, SES or any provider in production).
package mailer

import (
	"context"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/auth"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/people"
)

var Module = modules.Register(modules.Adapters, fx.Module("mailer", fx.Provide(
	New,
	func(m *Mailer) auth.Mailer { return m },
	func(m *Mailer) people.Mailer { return m },
)))

type Mailer struct {
	cfg config.Config
	log *slog.Logger
}

func New(cfg config.Config, log *slog.Logger) *Mailer { return &Mailer{cfg: cfg, log: log} }

// Send delivers a plain-text email. Without SMTP_HOST it only logs it, so password resets can be
// tried locally without a mail server.
func (m *Mailer) Send(ctx context.Context, to, subject, body string) error {
	if m.cfg.SMTPHost == "" {
		m.log.InfoContext(ctx, "email (SMTP not configured, not sent)", "to", to, "subject", subject, "body", body)
		return nil
	}
	from, err := mail.ParseAddress(m.cfg.MailFrom)
	if err != nil {
		return fmt.Errorf("MAIL_FROM: %w", err)
	}
	headers := []string{
		"From: " + from.String(),
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: 8bit",
	}
	msg := strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n")

	addr := net.JoinHostPort(m.cfg.SMTPHost, m.cfg.SMTPPort)
	var a smtp.Auth
	if m.cfg.SMTPUsername != "" {
		a = smtp.PlainAuth("", m.cfg.SMTPUsername, m.cfg.SMTPPassword, m.cfg.SMTPHost) // refuses plain text without TLS, except on localhost
	}
	// net/smtp takes no context: send in the background and stop waiting when the request is cancelled.
	done := make(chan error, 1)
	go func() { done <- smtp.SendMail(addr, a, from.Address, []string{to}, []byte(msg)) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
