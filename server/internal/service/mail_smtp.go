package service

import (
	"fmt"
	"log/slog"
	"strconv"

	"gopkg.in/gomail.v2"
)

// SMTPProvider sends email via an SMTP server using gomail.
// Supports STARTTLS (port 587) and implicit TLS (port 465).
type SMTPProvider struct {
	host     string
	port     int
	user     string
	password string
	useTLS   bool
}

func NewSMTPProviderFromEnv(host string, port int, user, password string, useTLS bool) *SMTPProvider {
	return &SMTPProvider{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		useTLS:   useTLS,
	}
}

func (p *SMTPProvider) Send(from, to, subject, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(p.host, p.port, p.user, p.password)
	// Port 465 uses implicit TLS (connect with TLS from the start).
	// Port 587 uses STARTTLS (connect plain, then upgrade).
	// For port 465, TLS is always required regardless of the useTLS flag.
	if p.port == 465 || p.useTLS {
		d.SSL = true
	}

	slog.Info("email: sending via SMTP", "host", p.host, "port", p.port, "from", from, "to", to)
	if err := d.DialAndSend(m); err != nil {
		slog.Error("email: SMTP send failed", "host", p.host, "port", p.port, "to", to, "error", err)
		return fmt.Errorf("smtp: dial and send: %w", err)
	}

	return nil
}

// parseSMTPPort parses a port string to int, returning defaultPort on failure.
func parseSMTPPort(s string, defaultPort int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 65535 {
		return defaultPort
	}
	return n
}
