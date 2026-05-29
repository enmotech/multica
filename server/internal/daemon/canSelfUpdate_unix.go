//go:build !windows

package daemon

import (
	"os"
	"path/filepath"
)

// canSelfUpdate reports whether the running daemon binary is writable by the
// current process. It returns false when the binary is installed in a
// root-owned directory (e.g. /usr/local/bin via sudo), meaning the daemon
// cannot replace itself and the Update button should be disabled.
func canSelfUpdate() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	resolved, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		resolved = exePath
	}
	return canSelfUpdatePath(resolved)
}

// canSelfUpdatePath checks write access to path by attempting to open it for
// writing without truncating. The file descriptor is closed immediately; no
// bytes are written. Returns false on any error (no such file, permission
// denied, etc.).
func canSelfUpdatePath(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}
