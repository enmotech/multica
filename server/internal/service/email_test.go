package service

import (
	"os"
	"strings"
	"testing"
)

func TestSanitizeSubjectField(t *testing.T) {
	long := strings.Repeat("a", 100)
	longRunes := strings.Repeat("深", 100)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain ascii", "Acme", "Acme"},
		{"strips newline", "Acme\nEvil", "AcmeEvil"},
		{"strips crlf header-style", "Acme\r\nBcc: evil@example.com", "AcmeBcc: evil@example.com"},
		{"strips tab", "Acme\tTeam", "AcmeTeam"},
		{"strips unicode control", "Acme\x07Beep", "AcmeBeep"},
		{"preserves non-ascii", "深度学习工作区", "深度学习工作区"},
		{"preserves emoji", "Team 🚀", "Team 🚀"},
		{"truncates long ascii", long, strings.Repeat("a", maxSubjectFieldRunes-1) + "…"},
		{"truncates rune-aware", longRunes, strings.Repeat("深", maxSubjectFieldRunes-1) + "…"},
		{"empty stays empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeSubjectField(tt.in)
			if got != tt.want {
				t.Errorf("sanitizeSubjectField(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEmailService_UsesProvider(t *testing.T) {
	var called bool
	mock := &stubProvider{SendFunc: func(from, to, subject, body string) error {
		called = true
		if to != "user@test.com" {
			t.Errorf("to = %q, want %q", to, "user@test.com")
		}
		if subject == "" {
			t.Error("subject should not be empty")
		}
		return nil
	}}
	svc := &EmailService{
		provider:  mock,
		fromEmail: "noreply@test.com",
	}
	err := svc.SendVerificationCode("user@test.com", "123456")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("provider.Send was not called")
	}
}

func TestEmailService_InvitationUsesProvider(t *testing.T) {
	var called bool
	mock := &stubProvider{SendFunc: func(from, to, subject, body string) error {
		called = true
		return nil
	}}
	svc := &EmailService{
		provider:  mock,
		fromEmail: "noreply@test.com",
	}
	err := svc.SendInvitationEmail("user@test.com", "Alice", "Acme", "inv-123")
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("provider.Send was not called")
	}
}

func TestEmailService_InvitationEscapesHTMLInBody(t *testing.T) {
	tests := []struct {
		name          string
		inviter       string
		workspace     string
		wantInBody    []string
		wantNotInBody []string
	}{
		{
			name:     "escapes script tag in inviter",
			inviter:  "<script>alert(1)</script>",
			workspace: "Acme",
			wantInBody: []string{
				"&lt;script&gt;alert(1)&lt;/script&gt;",
			},
			wantNotInBody: []string{
				"<script>alert(1)</script>",
			},
		},
		{
			name:     "escapes attribute-break payload in inviter",
			inviter:  `Alice" onclick="evil()`,
			workspace: "Acme",
			wantNotInBody: []string{
				`Alice" onclick="evil()`,
			},
		},
		{
			name:     "escapes anchor tag in workspace",
			inviter:  "Alice",
			workspace: `<a href="https://evil.example">Click</a>`,
			wantInBody: []string{
				"&lt;a href=",
				"&gt;Click&lt;/a&gt;",
			},
			wantNotInBody: []string{
				`<a href="https://evil.example">Click</a>`,
			},
		},
		{
			name:     "benign text unchanged",
			inviter:  "Alice",
			workspace: "Acme",
			wantInBody: []string{
				"Alice",
				"Acme",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedBody string
			mock := &stubProvider{SendFunc: func(from, to, subject, body string) error {
				capturedBody = body
				return nil
			}}
			os.Setenv("FRONTEND_ORIGIN", "https://test.com")
			defer os.Unsetenv("FRONTEND_ORIGIN")

			svc := &EmailService{provider: mock, fromEmail: "noreply@test.com"}
			err := svc.SendInvitationEmail("to@test.com", tt.inviter, tt.workspace, "abc-123")
			if err != nil {
				t.Fatal(err)
			}
			for _, needle := range tt.wantInBody {
				if !strings.Contains(capturedBody, needle) {
					t.Errorf("body missing %q\nbody: %s", needle, capturedBody)
				}
			}
			for _, needle := range tt.wantNotInBody {
				if strings.Contains(capturedBody, needle) {
					t.Errorf("body should not contain raw %q\nbody: %s", needle, capturedBody)
				}
			}
		})
	}
}

func TestEmailService_InvitationSubjectStripsControls(t *testing.T) {
	var capturedSubject string
	mock := &stubProvider{SendFunc: func(from, to, subject, body string) error {
		capturedSubject = subject
		return nil
	}}
	os.Setenv("FRONTEND_ORIGIN", "https://test.com")
	defer os.Unsetenv("FRONTEND_ORIGIN")

	svc := &EmailService{provider: mock, fromEmail: "noreply@test.com"}
	_ = svc.SendInvitationEmail("to@test.com", "Alice\r\n", "Acme\t", "abc")
	if strings.ContainsAny(capturedSubject, "\r\n\t") {
		t.Errorf("subject still contains control characters: %q", capturedSubject)
	}
	if capturedSubject != "Alice invited you to Acme on MoClaw" {
		t.Errorf("unexpected subject: %q", capturedSubject)
	}
}

func TestEmailService_InvitationSubjectNotHTMLEscaped(t *testing.T) {
	var capturedSubject string
	mock := &stubProvider{SendFunc: func(from, to, subject, body string) error {
		capturedSubject = subject
		return nil
	}}
	os.Setenv("FRONTEND_ORIGIN", "https://test.com")
	defer os.Unsetenv("FRONTEND_ORIGIN")

	svc := &EmailService{provider: mock, fromEmail: "noreply@test.com"}
	_ = svc.SendInvitationEmail("to@test.com", "Alice", "Acme & Co.", "abc")
	if strings.Contains(capturedSubject, "&amp;") {
		t.Errorf("subject should not be HTML-escaped, got %q", capturedSubject)
	}
	if !strings.Contains(capturedSubject, "Acme & Co.") {
		t.Errorf("subject missing literal ampersand: %q", capturedSubject)
	}
}

func TestEmailService_InvitationSubjectTruncated(t *testing.T) {
	var capturedSubject string
	mock := &stubProvider{SendFunc: func(from, to, subject, body string) error {
		capturedSubject = subject
		return nil
	}}
	os.Setenv("FRONTEND_ORIGIN", "https://test.com")
	defer os.Unsetenv("FRONTEND_ORIGIN")

	svc := &EmailService{provider: mock, fromEmail: "noreply@test.com"}
	longWorkspace := strings.Repeat("A", 200)
	_ = svc.SendInvitationEmail("to@test.com", "Alice", longWorkspace, "abc")
	maxExpected := len("Alice invited you to  on Multica") + maxSubjectFieldRunes
	if runes := len([]rune(capturedSubject)); runes > maxExpected {
		t.Errorf("subject not bounded: %d runes, max %d: %q", runes, maxExpected, capturedSubject)
	}
	if !strings.Contains(capturedSubject, "…") {
		t.Errorf("truncated subject should contain ellipsis marker: %q", capturedSubject)
	}
}

type stubProvider struct {
	SendFunc func(from, to, subject, body string) error
}

func (s *stubProvider) Send(from, to, subject, body string) error {
	return s.SendFunc(from, to, subject, body)
}
