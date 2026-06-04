//go:build windows

package serverinfo

import "golang.org/x/sys/windows"

const stillActive = 259 // STILL_ACTIVE

// processAlive reports whether pid refers to a live process by opening it and
// checking its exit code. A process that has exited reports a code != STILL_ACTIVE.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(h) }()

	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}
