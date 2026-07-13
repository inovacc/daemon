//go:build windows

package daemon

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// workerSysProcAttr returns the SysProcAttr used to launch the worker as a child of
// the (console-less) monitor. Unlike detachedSysProcAttr, this deliberately omits
// DETACHED_PROCESS: the worker must stay a child of the monitor so
// exec.CommandContext can Wait() on it and so `taskkill /T` still reaps the tree.
// CREATE_NO_WINDOW suppresses the phantom console Windows would otherwise allocate
// for a child of a console-less parent (the F1 bug); CREATE_NEW_PROCESS_GROUP puts
// the worker in its own process group.
//
// Graceful shutdown (F4) deliberately does NOT rely on
// GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, ...) here even though
// CREATE_NEW_PROCESS_GROUP would make that possible in principle: CTRL events
// require the sender and receiver to share a console, which CREATE_NO_WINDOW
// breaks (it gives the worker its own, merely-hidden console) — confirmed
// empirically (a live spawn+signal probe: the call succeeds but the child never
// reacts) — and is fragile across deployment topologies more generally (e.g. a
// monitor whose own stdio is redirected, or run with no console at all). Instead,
// prepareGracefulShutdown (worker_signal_windows.go) uses a named Windows event,
// which has no console dependency at all.
func workerSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}
