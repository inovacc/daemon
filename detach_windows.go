//go:build windows

package daemon

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// detachedSysProcAttr returns the SysProcAttr used to launch a fully detached child:
// no console window, its own process group, not tied to the parent's lifetime. Shared
// by spawnDetached (background monitor) and reexecSelf (in-place upgrade) so the two
// launch paths cannot drift apart.
func detachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW,
		HideWindow:    true,
	}
}
