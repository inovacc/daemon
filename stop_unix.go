//go:build !windows

package daemon

import "syscall"

// stopProcess terminates the daemon. The monitor is a session leader (Setsid), so a
// negative pid signals the whole process group (monitor + worker); on failure it
// falls back to signalling the single process.
func stopProcess(pid int) error {
	if err := syscall.Kill(-pid, syscall.SIGTERM); err == nil {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGTERM)
}
