//go:build !windows

package daemon

import "syscall"

// workerSysProcAttr returns the SysProcAttr used to launch the worker as a child of
// the monitor: its own process group (Setpgid) so a signal can target the worker
// (and any of its own children) without also hitting the monitor.
func workerSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
