//go:build !windows && !linux && !darwin

package serverinfo

// processAlive reports whether pid refers to a live process, using signal 0.
//
// This is the fallback for unix platforms without a dedicated implementation
// (linux uses /proc, darwin uses sysctl — see proc_linux.go / proc_darwin.go).
// It handles EPERM correctly (see signalZero) but canNOT see zombies: an exited,
// unreaped CHILD still answers kill(pid, 0). That is acceptable here because the
// production Stop() path targets the monitor pid recorded in server.json, which is
// a detached process the stopping CLI is not the parent of — so it can never be a
// zombie from that process's point of view.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	return signalZero(pid)
}
