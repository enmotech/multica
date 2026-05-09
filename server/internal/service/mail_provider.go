package service

import (
	"fmt"
	"io"
	"os"
)

// Provider abstracts email sending. Implementations handle specific backends
// (Resend API, SMTP, log-to-stdout for dev).
type Provider interface {
	// Send delivers an email. from, to, subject are plain text; body is HTML.
	Send(from, to, subject, body string) error
}

// LogProvider writes emails to a Writer (stdout in production). Used when no
// email backend is configured — the existing dev-mode behavior.
type LogProvider struct {
	Writer io.Writer
}

func (p *LogProvider) Send(from, to, subject, body string) error {
	w := p.Writer
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, "[DEV EMAIL] from=%s to=%s subject=%q\n", from, to, subject)
	return nil
}
