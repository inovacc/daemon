//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"syscall"
)

// reexecSelf replaces the current process image in place with a fresh exec of the
// same binary on disk, forwarding args (the original os.Args[1:] from the caller).
// On success it does NOT return — the running image is gone and the new image takes
// over with the same PID. It returns an error only when locating the executable or
// the exec syscall itself fails.
func reexecSelf(args []string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}

	argv := append([]string{self}, args...)

	return syscall.Exec(self, argv, os.Environ())
}
