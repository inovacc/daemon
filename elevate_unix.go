//go:build !windows

package daemon

import "os"

// isElevated reports whether the process runs with root privileges (effective uid 0).
func isElevated() bool {
	return os.Geteuid() == 0
}
