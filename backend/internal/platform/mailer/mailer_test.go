package mailer

import (
	"bufio"
	"bytes"
	"context"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/reihnxx/go-react-boilerplate/backend/internal/config"
)

func TestLogMailerRedactsInProduction(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	msg := Message{To: "a@example.com", Subject: "Reset", Text: "token=SECRET123"}

	_ = New(config.MailConfig{Driver: "log"}, logger).Send(context.Background(), msg)
	if !strings.Contains(buf.String(), "SECRET123") {
		t.Fatal("development log driver should include the body")
	}
	buf.Reset()
	_ = New(config.MailConfig{Driver: "log", RedactLog: true}, logger).Send(context.Background(), msg)
	if strings.Contains(buf.String(), "SECRET123") {
		t.Fatal("production log driver leaked the body")
	}
}

// fakeSMTP is a minimal SMTP server that records one message.
func fakeSMTP(t *testing.T) (addr string, received chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	received = make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		r := bufio.NewReader(conn)
		write := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
		write("220 fake ESMTP")
		var data strings.Builder
		inData := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if line == ".\r\n" {
					inData = false
					received <- data.String()
					write("250 OK")
					continue
				}
				data.WriteString(line)
				continue
			}
			switch cmd := strings.ToUpper(strings.TrimSpace(line)); {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				write("250 fake")
			case strings.HasPrefix(cmd, "MAIL"), strings.HasPrefix(cmd, "RCPT"), strings.HasPrefix(cmd, "RSET"), strings.HasPrefix(cmd, "NOOP"):
				write("250 OK")
			case cmd == "DATA":
				inData = true
				write("354 go ahead")
			case cmd == "QUIT":
				write("221 bye")
				return
			default:
				write("502 unknown")
			}
		}
	}()
	return ln.Addr().String(), received
}

func TestSMTPMailerDelivers(t *testing.T) {
	addr, received := fakeSMTP(t)
	host, port, _ := net.SplitHostPort(addr)
	p, _ := strconv.Atoi(port)
	m := New(config.MailConfig{Driver: "smtp", Host: host, Port: p, From: "noreply@example.com"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.Send(ctx, Message{To: "user@example.com", Subject: "Halo", Text: "Isi pesan"}); err != nil {
		t.Fatal(err)
	}
	select {
	case data := <-received:
		for _, want := range []string{"From: noreply@example.com", "To: user@example.com", "Subject: Halo", "Isi pesan"} {
			if !strings.Contains(data, want) {
				t.Errorf("message missing %q:\n%s", want, data)
			}
		}
	case <-ctx.Done():
		t.Fatal("no message received")
	}
}

func TestSMTPMailerRejectsHeaderInjection(t *testing.T) {
	m := New(config.MailConfig{Driver: "smtp", Host: "127.0.0.1", Port: 1, From: "x@example.com"}, nil)
	err := m.Send(context.Background(), Message{To: "a@example.com\r\nBcc: victim@example.com", Subject: "x", Text: "y"})
	if err == nil {
		t.Fatal("expected header injection to be rejected")
	}
}
