//go:build windows

package daemon

import "golang.org/x/sys/windows"

// isElevated reports whether the current process token is elevated (Administrator).
// GetCurrentProcessToken returns a pseudo-token that needs no Close; IsElevated reads the
// token's elevation flag (TokenElevation). Verified against golang.org/x/sys v0.34.0, the
// same module/version already imported by spawn_windows.go.
func isElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
