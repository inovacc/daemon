//go:build windows

package daemon

import (
	"os/exec"
	"strconv"
)

// stopProcess kills the process tree rooted at pid (taskkill /T kills children too).
func stopProcess(pid int) error {
	return exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
}
