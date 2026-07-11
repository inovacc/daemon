//go:build windows

package daemon

import (
	"fmt"
	"os"
	"os/exec"
)

// reexecSelf spawns a fresh DETACHED child running the (new) binary image with the
// original os.Args[1:], then exits the current process so the new image supersedes
// the old. Windows has no exec(); this is the equivalent of the Unix in-place replace.
// It reuses the detached CreationFlags from spawn_windows.go. On success it does NOT
// return (os.Exit terminates the process); it returns an error only when locating the
// executable or starting the child fails.
func reexecSelf(args []string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}

	cmd := exec.Command(self, args...)
	cmd.Env = os.Environ()
	// DETACHED_PROCESS + CREATE_NO_WINDOW drop stdio, so nil-ing it is unnecessary.
	cmd.SysProcAttr = detachedSysProcAttr()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn upgraded monitor: %w", err)
	}

	_ = cmd.Process.Release()

	os.Exit(0)

	return nil // unreachable; os.Exit does not return, but the compiler requires it.
}
