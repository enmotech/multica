package service

import (
	"testing"
)

func TestSMTPProvider_ImplementsInterface(t *testing.T) {
	// Verify SMTPProvider implements Provider at compile time.
	var _ Provider = (*SMTPProvider)(nil)
}

func TestNewSMTPProviderFromEnv_Defaults(t *testing.T) {
	p := NewSMTPProviderFromEnv(
		"smtp.example.com",
		587,
		"user@example.com",
		"password",
		true,
	)
	if p == nil {
		t.Fatal("expected non-nil SMTPProvider")
	}
	if p.host != "smtp.example.com" {
		t.Errorf("host = %q, want %q", p.host, "smtp.example.com")
	}
	if p.port != 587 {
		t.Errorf("port = %d, want %d", p.port, 587)
	}
}

func TestParseSMTPPort(t *testing.T) {
	tests := []struct {
		input    string
		def      int
		expected int
	}{
		{"587", 25, 587},
		{"465", 25, 465},
		{"", 25, 25},
		{"abc", 25, 25},
		{"0", 25, 25},
		{"99999", 25, 25},
		{"-1", 25, 25},
	}
	for _, tt := range tests {
		got := parseSMTPPort(tt.input, tt.def)
		if got != tt.expected {
			t.Errorf("parseSMTPPort(%q, %d) = %d, want %d", tt.input, tt.def, got, tt.expected)
		}
	}
}
