//go:build !windows

package daemon

import (
	"os/exec"
	"syscall"
)

// spawnDetached starts exe in a new session (setsid), detached from the controlling
// terminal, with stdio discarded. Returns the child pid.
func spawnDetached(exe string, args, env []string) (int, error) {
	cmd := exec.Command(exe, args...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	pid := cmd.Process.Pid
	_ = cmd.Process.Release()

	return pid, nil
}
