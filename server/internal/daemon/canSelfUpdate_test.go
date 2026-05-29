//go:build !windows

package daemon

import (
	"os"
	"path/filepath"
	"runtime"
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

// TestCanSelfUpdatePath_runningBinary verifies that the currently-executing test
// binary is reported as writable. On Linux, opening a running executable with
// O_WRONLY returns ETXTBSY (text file busy) rather than EACCES — the kernel
// signals "you have permission but the file is in use", not "permission denied".
// canSelfUpdatePath must treat ETXTBSY as writable (true), because the update
// path replaces the binary via rename, not by writing through an open fd.
func TestCanSelfUpdatePath_runningBinary(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("ETXTBSY is Linux-specific; other platforms allow O_WRONLY on running binaries")
	}
	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		resolved = exePath
	}
	if !canSelfUpdatePath(resolved) {
		t.Errorf("canSelfUpdatePath(%q) = false; want true — running binary is writable (ETXTBSY must not be treated as permission denied)", resolved)
	}
}
