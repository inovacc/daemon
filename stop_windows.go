//go:build windows

package daemon

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// stopProcess kills the process tree rooted at pid (taskkill /T kills children too),
// then confirms the process actually exited before returning. It folds taskkill's
// output into the error (mirroring runSchtasks) so the caller sees why the OS
// refused — e.g. "Access is denied" without admin — instead of a bare exit-status
// error.
func stopProcess(pid int) error {
	out, err := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").CombinedOutput()
	if err != nil {
		return fmt.Errorf("taskkill pid %d: %w: %s", pid, err, strings.TrimSpace(string(out)))
	}

	return waitForProcessExit(pid, stopConfirmTimeout)
}
