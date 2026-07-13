package daemon

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/inovacc/daemon/internal/serverinfo"
)

// spawnSleeper starts this test binary in "sleeper" role (blocks forever until
// killed) and returns the running *exec.Cmd. It gives F5's waitForProcessExit a
// real, killable OS process instead of a faked pid.
func spawnSleeper(t *testing.T) *exec.Cmd {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	cmd := exec.Command(self)

	cmd.Env = append(os.Environ(), itSleepEnv+"=1")

	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn sleeper: %v", err)
	}

	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	return cmd
}

// F5: stopProcess (Windows taskkill / Unix kill) must confirm the process actually
// exited before returning, not just that the kill/signal call itself succeeded.
// This test pins the shared confirmation helper, waitForProcessExit.
func TestWaitForProcessExitDetectsExit(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a real process; skipped under -short")
	}

	cmd := spawnSleeper(t)
	pid := cmd.Process.Pid

	if !serverinfo.ProcessAlive(pid) {
		t.Fatalf("sleeper pid %d should be alive right after spawn", pid)
	}

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill sleeper: %v", err)
	}

	if err := waitForProcessExit(pid, 5*time.Second); err != nil {
		t.Fatalf("waitForProcessExit after kill: %v", err)
	}

	_ = cmd.Wait()
}

// F5: waitForProcessExit must return an error (not a false "stopped") when the
// process is still alive at the bound.
func TestWaitForProcessExitTimesOutWhileAlive(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a real process; skipped under -short")
	}

	cmd := spawnSleeper(t)

	if err := waitForProcessExit(cmd.Process.Pid, 150*time.Millisecond); err == nil {
		t.Fatal("waitForProcessExit must time out while the process is still alive")
	}
}
