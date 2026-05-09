package service

import (
	"github.com/resend/resend-go/v2"
)

// ResendProvider sends email via the Resend API.
type ResendProvider struct {
	client *resend.Client
}

func NewResendProvider(apiKey string) *ResendProvider {
	return &ResendProvider{
		client: resend.NewClient(apiKey),
	}
}

func (p *ResendProvider) Send(from, to, subject, body string) error {
	_, err := p.client.Emails.Send(&resend.SendEmailRequest{
		From:    from,
		To:      []string{to},
		Subject: subject,
		Html:    body,
	})
	return err
}
