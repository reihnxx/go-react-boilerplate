// Package mailer sends transactional email. The "log" driver writes messages
// to the application log and is meant for local development only.
package mailer

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/config"
)

type Message struct {
	To      string
	Subject string
	Text    string
}

type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

func New(cfg config.MailConfig, logger *slog.Logger) Mailer {
	if cfg.Driver == "smtp" {
		return &SMTP{cfg: cfg}
	}
	return &Log{Logger: logger, Redact: cfg.RedactLog}
}

// Log prints emails to the logger instead of sending them. With Redact the
// body is omitted, because it can contain secrets such as reset links.
type Log struct {
	Logger *slog.Logger
	Redact bool
}

func (l *Log) Send(ctx context.Context, msg Message) error {
	body := msg.Text
	if l.Redact {
		body = "[redacted: configure MAIL_DRIVER=smtp to deliver mail]"
	}
	l.Logger.InfoContext(ctx, "mail (log driver, not sent)", "to", msg.To, "subject", msg.Subject, "body", body)
	return nil
}

// SMTP sends plain-text mail via net/smtp (STARTTLS when the server offers it).
type SMTP struct{ cfg config.MailConfig }

func (s *SMTP) Send(ctx context.Context, msg Message) error {
	if strings.ContainsAny(msg.To+msg.Subject, "\r\n") {
		return fmt.Errorf("mailer: header injection attempt")
	}
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	var auth smtp.Auth
	if s.cfg.User != "" {
		auth = smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)
	}
	body := strings.Join([]string{
		"From: " + s.cfg.From,
		"To: " + msg.To,
		"Subject: " + msg.Subject,
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		msg.Text,
	}, "\r\n")

	done := make(chan error, 1)
	go func() { done <- smtp.SendMail(addr, auth, s.cfg.From, []string{msg.To}, []byte(body)) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
