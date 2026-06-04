package daemon

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// ErrNeedsPrivilege is returned by RequirePrivilege when a privileged svc verb is invoked
// without admin/root. ExitCodeFor maps it (even when wrapped) to ExitNeedsPrivilege (5).
var ErrNeedsPrivilege = errors.New("daemon: operation requires administrator/root privileges")

// RequirePrivilege is the FIRST call in every privileged svc verb's RunE (svc install/
// uninstall/start/stop/restart). When the process is NOT elevated it writes platform-specific
// re-run guidance — built from cmd.CommandPath() — to cmd.ErrOrStderr() and returns
// ErrNeedsPrivilege WITHOUT performing the operation and WITHOUT re-launching elevated
// (detect-and-guide only). When elevated it returns nil and the verb proceeds.
func RequirePrivilege(cmd *cobra.Command) error {
	if isElevatedFn() {
		return nil
	}
	w := cmd.ErrOrStderr()
	path := cmd.CommandPath() // e.g. "daemon svc install"
	switch runtime.GOOS {
	case "windows":
		_, _ = fmt.Fprintf(w, "%q requires administrator privileges.\n", path)
		_, _ = fmt.Fprintln(w, "Re-run from an elevated PowerShell, for example:")
		_, _ = fmt.Fprintf(w, "  Start-Process powershell -Verb RunAs -ArgumentList '%s'\n", path)
	default:
		_, _ = fmt.Fprintf(w, "%q requires root privileges.\n", path)
		_, _ = fmt.Fprintln(w, "Re-run with sudo, for example:")
		_, _ = fmt.Fprintf(w, "  sudo %s\n", path)
	}
	return ErrNeedsPrivilege
}
