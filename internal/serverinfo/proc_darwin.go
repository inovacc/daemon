//go:build darwin

package serverinfo

import "golang.org/x/sys/unix"

// szomb is SZOMB from <sys/proc.h>: the process has exited and is awaiting a reap
// by its parent (a zombie). Not exported by golang.org/x/sys/unix, so it is
// spelled out here.
const szomb = 5

// processAlive reports whether pid refers to a live process.
//
// It queries kern.proc.pid via sysctl rather than using kill(pid, 0), because
// kill(pid, 0) cannot distinguish a live process from a ZOMBIE — a child that has
// exited but whose parent has not reaped it. A zombie keeps its pid until reaped
// and answers kill(pid, 0) successfully. Since this library's own Start() spawns
// the monitor as a child (spawnDetached: cmd.Start + Release, which does NOT
// reap), a "poll until the pid is gone" loop built on kill(pid, 0) would hang
// forever against a zombie monitor. Treating a zombie as NOT running is the
// correct answer for every caller here: the process is dead, it is merely awaiting
// a reap.
//
// For a pid that does not exist, the sysctl returns a zero-length result, which
// SysctlKinfoProc surfaces as an error — so an error means "no such process".
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil || kp == nil {
		return false
	}

	return kp.Proc.P_stat != szomb
}
