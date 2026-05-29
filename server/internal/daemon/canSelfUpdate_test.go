//go:build !windows

package daemon

import (
	"os"
	"testing"
)

// TestCanSelfUpdatePath_writableFile verifies that a writable binary path
// reports can-self-update = true.
func TestCanSelfUpdatePath_writableFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "multica-binary-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	if !canSelfUpdatePath(f.Name()) {
		t.Error("expected true for a writable temp file")
	}
}

// TestCanSelfUpdatePath_readonlyFile verifies that a 0444 binary path (common
// when installed via sudo into /usr/local/bin) reports can-self-update = false.
func TestCanSelfUpdatePath_readonlyFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "multica-binary-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0o444); err != nil {
		t.Fatal(err)
	}

	if canSelfUpdatePath(f.Name()) {
		t.Error("expected false for a read-only file (0444)")
	}
}

// TestCanSelfUpdatePath_nonexistent verifies that a missing path returns false
// rather than panicking.
func TestCanSelfUpdatePath_nonexistent(t *testing.T) {
	if canSelfUpdatePath("/nonexistent/path/to/multica-binary") {
		t.Error("expected false for a nonexistent path")
	}
}
