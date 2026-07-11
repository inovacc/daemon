//go:build windows

package daemon

import "os/exec"

// spawnDetached starts exe fully detached: no console window, its own process group,
// not tied to the parent's lifetime. Returns the child pid.
func spawnDetached(exe string, args, env []string) (int, error) {
	cmd := exec.Command(exe, args...)
	cmd.Env = env

	cmd.SysProcAttr = detachedSysProcAttr()
	if err := cmd.Start(); err != nil {
		return 0, err
	}

	pid := cmd.Process.Pid
	_ = cmd.Process.Release()

	return pid, nil
}
