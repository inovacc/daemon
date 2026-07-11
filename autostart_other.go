//go:build !windows

package daemon

import (
	"fmt"
	"runtime"
)

// realAutostartManager on non-Windows platforms reports that the logon-autostart
// verbs are Windows-only. The daemon's cross-platform background lifecycle lives
// under the `svc` group (kardianos: systemd / launchd / Windows service); the
// `autostart` group specifically reflects the Windows Startup Run-key and Task
// Scheduler mechanisms, which have no portable equivalent here.
func realAutostartManager(_ Options, _ []string) (autostartManager, error) {
	return nil, fmt.Errorf(
		"daemon: autostart (Startup/Task Scheduler) is Windows-only; on %s use the `svc` command (systemd/launchd) instead",
		runtime.GOOS,
	)
}
