package service

import (
	"testing"
)

func TestResendProvider_ImplementsInterface(t *testing.T) {
	// Verify that ResendProvider implements the Provider interface at compile time.
	var _ Provider = (*ResendProvider)(nil)
}
