//go:build linux

package serverinfo

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"strconv"
)

// procSelfStat is read to distinguish "this pid is gone" from "/proc is not
// mounted at all" (some minimal containers). A package var so tests can point it
// at a path that does not exist.
var procSelfStat = "/proc/self/stat"

// processAlive reports whether pid refers to a live process.
//
// It reads /proc/<pid>/stat rather than using kill(pid, 0), because kill(pid, 0)
// cannot distinguish a live process from a ZOMBIE — a child that has exited but
// whose parent has not reaped it. A zombie keeps its pid (and its /proc entry)
// until reaped, and answers kill(pid, 0) successfully. Since this library's own
// Start() spawns the monitor as a child (spawnDetached: cmd.Start + Release, which
// does NOT reap), a "poll until the pid is gone" loop built on kill(pid, 0) would
// hang forever against a zombie monitor. Treating a zombie as NOT running is the
// correct answer for every caller here: the process is dead, it is merely awaiting
// a reap. /proc also reports another user's process without EPERM trouble.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err == nil {
		return !isDeadProcState(data)
	}

	if errors.Is(err, fs.ErrNotExist) {
		// Either the process is genuinely gone, or /proc is not mounted. Only the
		// first is a "dead" answer, so disambiguate with our OWN entry: if that is
		// missing too, /proc is unavailable and we must fall back to kill(pid, 0)
		// rather than declare every process dead.
		if _, selfErr := os.Stat(procSelfStat); selfErr != nil {
			return signalZero(pid)
		}

		return false
	}

	// Any other read error (permissions, transient): fall back rather than guess.
	return signalZero(pid)
}

// isDeadProcState reports whether a /proc/<pid>/stat line's state field marks the
// process as no longer running: 'Z' (zombie — exited, awaiting reap) or 'X'/'x'
// (dead). See proc(5).
//
// The state is the third field, but the second (comm) is wrapped in parentheses
// and may itself contain spaces AND parentheses, so the fields before it cannot be
// split on whitespace. Scanning from the LAST ')' is the documented-safe parse.
func isDeadProcState(data []byte) bool {
	i := bytes.LastIndexByte(data, ')')
	if i < 0 || i+2 >= len(data) {
		return false
	}

	switch data[i+2] {
	case 'Z', 'X', 'x':
		return true
	default:
		return false
	}
}
