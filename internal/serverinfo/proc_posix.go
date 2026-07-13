// Excludes darwin: that implementation answers purely from sysctl (which reports a
// zombie's state directly and needs no kill(2) fallback), so signalZero would be
// dead code there.
//go:build !windows && !darwin

package serverinfo

import (
	"errors"
	"syscall"
)

// signalZero reports liveness via the classic kill(pid, 0) probe.
//
// EPERM means the process EXISTS but we are not permitted to signal it (e.g. the
// monitor runs as root/another user and we do not) — that is ALIVE, not dead.
// Only ESRCH ("no such process") means gone. Reporting an EPERM process as dead
// would make Stop's exit-confirmation claim success against a process it never
// actually killed.
//
// NOTE: this probe canNOT see zombies — a child that has exited but has not been
// reaped by its parent still answers kill(pid, 0) successfully. It is therefore
// only used as a FALLBACK; the linux and darwin implementations of processAlive
// use a state-aware source (/proc and sysctl respectively) so that a zombie is
// correctly reported dead. See proc_linux.go / proc_darwin.go.
func signalZero(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}

	return errors.Is(err, syscall.EPERM)
}
