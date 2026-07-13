//go:build !windows

package daemon

import (
	"errors"
	"syscall"
)

// stopProcess terminates the daemon. The monitor is a session leader (Setsid), so a
// negative pid signals the whole process group (monitor + worker); on failure it
// falls back to signalling the single process. When BOTH the group and single-pid
// kills fail, their errors are joined so the caller sees why (EPERM vs ESRCH) instead
// of only the fallback's error. Once the signal is delivered, it confirms the
// process actually exited before returning.
func stopProcess(pid int) error {
	groupErr := syscall.Kill(-pid, syscall.SIGTERM)
	if groupErr != nil {
		if singleErr := syscall.Kill(pid, syscall.SIGTERM); singleErr != nil {
			return errors.Join(groupErr, singleErr)
		}
	}

	return waitForProcessExit(pid, stopConfirmTimeout)
}
