package service

import (
	"fmt"
	"html"
	"log/slog"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxSubjectFieldRunes = 60

type EmailService struct {
	provider  Provider
	fromEmail string
}

// NewEmailService creates an EmailService with a Provider selected from
// environment variables:
//   - EMAIL_PROVIDER=smtp + SMTP_* vars → SMTPProvider
//   - EMAIL_PROVIDER=resend (default) + RESEND_API_KEY → ResendProvider
//   - Neither configured → LogProvider (prints to stdout)
func NewEmailService() *EmailService {
	provider := resolveProvider()

	from := os.Getenv("RESEND_FROM_EMAIL")
	if from == "" {
		// SMTP servers often require the From address to match the
		// authenticated user. Fall back to SMTP_USER so self-hosted
		// instances work out of the box without setting RESEND_FROM_EMAIL.
		if os.Getenv("EMAIL_PROVIDER") == "smtp" {
			from = os.Getenv("SMTP_USER")
		}
	}
	if from == "" {
		from = "noreply@multica.ai"
	}

	return &EmailService{
		provider:  provider,
		fromEmail: from,
	}
}

// NewEmailServiceWithProvider creates an EmailService with an explicit provider.
// Used in tests.
func NewEmailServiceWithProvider(provider Provider, fromEmail string) *EmailService {
	return &EmailService{
		provider:  provider,
		fromEmail: fromEmail,
	}
}

func resolveProvider() Provider {
	switch os.Getenv("EMAIL_PROVIDER") {
	case "smtp":
		host := os.Getenv("SMTP_HOST")
		if host == "" {
			slog.Warn("email: EMAIL_PROVIDER=smtp but SMTP_HOST is empty; falling back to log provider")
			return &LogProvider{}
		}
		port := parseSMTPPort(os.Getenv("SMTP_PORT"), 587)
		user := os.Getenv("SMTP_USER")
		password := os.Getenv("SMTP_PASSWORD")
		useTLS := os.Getenv("SMTP_TLS") != "false"
		slog.Info("email: using SMTP provider", "host", host, "port", port, "user", user, "tls", useTLS)
		return NewSMTPProviderFromEnv(host, port, user, password, useTLS)
	default:
		apiKey := os.Getenv("RESEND_API_KEY")
		if apiKey != "" {
			slog.Info("email: using Resend provider")
			return NewResendProvider(apiKey)
		}
		slog.Info("email: no provider configured; using log provider (emails print to stdout)")
		return &LogProvider{}
	}
}

func (s *EmailService) SendVerificationCode(to, code string) error {
	subject := "Your MoClaw verification code"
	body := fmt.Sprintf(
		`<div style="font-family: sans-serif; max-width: 400px; margin: 0 auto;">
			<h2>Your verification code</h2>
			<p style="font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 24px 0;">%s</p>
			<p>This code expires in 10 minutes.</p>
			<p style="color: #666; font-size: 14px;">If you didn't request this code, you can safely ignore this email.</p>
		</div>`, code)
	return s.provider.Send(s.fromEmail, to, subject, body)
}

func (s *EmailService) SendInvitationEmail(to, inviterName, workspaceName, invitationID string) error {
	appURL := strings.TrimSpace(os.Getenv("FRONTEND_ORIGIN"))
	if appURL == "" {
		appURL = "https://app.multica.ai"
	}
	inviteURL := fmt.Sprintf("%s/invite/%s", appURL, invitationID)

	safeWorkspace := html.EscapeString(workspaceName)
	safeInviter := html.EscapeString(inviterName)
	subjectInviter := sanitizeSubjectField(inviterName)
	subjectWorkspace := sanitizeSubjectField(workspaceName)

	subject := fmt.Sprintf("%s invited you to %s on MoClaw", subjectInviter, subjectWorkspace)
	body := fmt.Sprintf(
		`<div style="font-family: sans-serif; max-width: 480px; margin: 0 auto;">
			<h2>You're invited to join %s</h2>
			<p><strong>%s</strong> invited you to collaborate in the <strong>%s</strong> workspace on MoClaw.</p>
			<p style="margin: 24px 0;">
				<a href="%s" style="display: inline-block; padding: 12px 24px; background: #000; color: #fff; text-decoration: none; border-radius: 6px; font-weight: 500;">Accept invitation</a>
			</p>
			<p style="color: #666; font-size: 14px;">You'll need to log in to accept or decline the invitation.</p>
		</div>`, safeWorkspace, safeInviter, safeWorkspace, inviteURL)

	return s.provider.Send(s.fromEmail, to, subject, body)
}

func sanitizeSubjectField(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	cleaned := b.String()
	if utf8.RuneCountInString(cleaned) <= maxSubjectFieldRunes {
		return cleaned
	}
	runes := []rune(cleaned)
	return string(runes[:maxSubjectFieldRunes-1]) + "…"
}
