package daemon

import "errors"

// ExitStatus is the monitor<->worker exit-code protocol. The worker communicates its
// intent to the monitor purely through its process exit code.
type ExitStatus int

const (
	// ExitSuccess is a clean shutdown: the monitor exits, no restart.
	ExitSuccess ExitStatus = 0
	// ExitError is a crash: the monitor restarts the worker (subject to the fork-loop guard).
	ExitError ExitStatus = 1
	// ExitRestart is an intentional restart (e.g. config change via API). The monitor
	// re-loops WITHOUT counting it as a crash and WITHOUT applying restart backoff.
	ExitRestart ExitStatus = 3
	// ExitUpgrade signals the binary was replaced on disk; the monitor re-exec's itself
	// (syscall.Exec on Unix, spawn on Windows).
	ExitUpgrade ExitStatus = 4
	// ExitNeedsPrivilege is returned when a privileged svc verb (svc install/uninstall/
	// start/stop/restart) is invoked without admin/root. The process prints guidance and
	// aborts WITHOUT attempting the operation and WITHOUT re-launching elevated.
	ExitNeedsPrivilege ExitStatus = 5
)

// AsInt returns the exit code as a plain int for os.Exit.
func (e ExitStatus) AsInt() int { return int(e) }

// ExitCodeFor maps an error returned from command execution to a process exit code.
// nil -> ExitSuccess (0); ErrNeedsPrivilege (even when wrapped) -> ExitNeedsPrivilege (5);
// any other error -> ExitError (1). New mappings may be ADDED here; existing codes are
// never reused.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitSuccess.AsInt()
	}
	if errors.Is(err, ErrNeedsPrivilege) {
		return ExitNeedsPrivilege.AsInt()
	}
	return ExitError.AsInt()
}
