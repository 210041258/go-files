//go:build windows
// +build windows

package testutils

// InstallGoroutineDumpSignal does nothing on Windows.
// There is no equivalent to SIGUSR1; this function is provided for
// cross‑platform compatibility.
func InstallGoroutineDumpSignal() {
	// No‑op
}

// InstallQuitSignalDump does nothing on Windows.
// Ctrl+C is handled elsewhere; Ctrl+Break is not SIGQUIT.
func InstallQuitSignalDump() {
	// No‑op
}
