package daemon

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
)

// AsInt returns the exit code as a plain int for os.Exit.
func (e ExitStatus) AsInt() int { return int(e) }
