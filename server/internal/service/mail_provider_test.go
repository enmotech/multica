package service

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogProvider_Send(t *testing.T) {
	var buf bytes.Buffer
	p := &LogProvider{Writer: &buf}
	err := p.Send("from@test.com", "to@test.com", "Test Subject", "<h1>Hello</h1>")
	if err != nil {
		t.Fatalf("LogProvider.Send() error = %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "to@test.com") {
		t.Errorf("log output missing recipient: %q", output)
	}
	if !strings.Contains(output, "Test Subject") {
		t.Errorf("log output missing subject: %q", output)
	}
}
