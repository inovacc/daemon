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
	// ExitAlreadyRunning is returned by `service start` (ErrAlreadyRunning) when a
	// live instance already exists.
	ExitAlreadyRunning ExitStatus = 6
	// ExitNotRunning is returned by `service stop` / `service status`
	// (ErrNotRunning) when no live instance exists.
	ExitNotRunning ExitStatus = 7
)

// AsInt returns the exit code as a plain int for os.Exit.
func (e ExitStatus) AsInt() int { return int(e) }

// ExitCodeFor maps an error returned from command execution to a process exit code.
// nil -> ExitSuccess (0); ErrNeedsPrivilege (even when wrapped) -> ExitNeedsPrivilege
// (5); ErrAlreadyRunning -> ExitAlreadyRunning (6); ErrNotRunning -> ExitNotRunning
// (7); any other error -> ExitError (1). New mappings may be ADDED here; existing
// codes are never reused.
//
// BREAKING (0.2.0): `service status`/`service stop` against an idle host, and
// `service start` against a live one, now exit non-zero (6/7) instead of 0 — see
// F7 in CHANGELOG.md. Route the hidden monitor/worker commands through this
// function too (see IsSupervisorCommand) so their exit codes stay in the
// monitor<->worker protocol rather than leaking a consumer's own exit-code
// contract into it.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitSuccess.AsInt()
	}

	switch {
	case errors.Is(err, ErrNeedsPrivilege):
		return ExitNeedsPrivilege.AsInt()
	case errors.Is(err, ErrAlreadyRunning):
		return ExitAlreadyRunning.AsInt()
	case errors.Is(err, ErrNotRunning):
		return ExitNotRunning.AsInt()
	default:
		return ExitError.AsInt()
	}
}
