//go:build windows

package daemon

// canSelfUpdate always returns true on Windows. The daemon does not run in
// production on Windows; this stub keeps the build green on all platforms.
func canSelfUpdate() bool { return true }

// canSelfUpdatePath always returns true on Windows (see canSelfUpdate).
func canSelfUpdatePath(_ string) bool { return true }
