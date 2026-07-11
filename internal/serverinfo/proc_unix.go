//go:build !windows

package serverinfo

import (
	"os"
	"syscall"
)

// processAlive reports whether pid refers to a live process, using signal 0.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
