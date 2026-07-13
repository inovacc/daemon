package daemon

import (
	"fmt"
	"time"

	"github.com/inovacc/daemon/internal/serverinfo"
)

// stopConfirmTimeout bounds how long stopProcess waits for the OS to actually reap
// pid after the kill/signal was issued, before giving up and reporting failure. A
// package var so tests can shrink it.
var stopConfirmTimeout = 5 * time.Second

// stopConfirmPoll is the interval between liveness checks while waiting for pid to
// exit. A package var so tests can shrink it.
var stopConfirmPoll = 50 * time.Millisecond

// waitForProcessExit polls pid's liveness until it is gone or timeout elapses. It is
// shared by the Windows (taskkill) and Unix (kill) stopProcess implementations so
// "stop" confirms the process is actually dead instead of merely having issued the
// kill request.
func waitForProcessExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		if !serverinfo.ProcessAlive(pid) {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("daemon: pid %d still alive %s after stop was issued", pid, timeout)
		}

		time.Sleep(stopConfirmPoll)
	}
}
