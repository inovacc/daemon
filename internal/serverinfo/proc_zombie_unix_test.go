//go:build !windows

package serverinfo

import (
	"os/exec"
	"testing"
	"time"
)

// TestProcessAliveZombieChildIsNotAlive pins the Unix zombie trap that broke CI on
// ubuntu/macOS: a child process that has EXITED but has not been reaped by its
// parent is a zombie. It keeps its pid and still answers kill(pid, 0)
// successfully, so a naive liveness probe reports it as alive forever — and any
// "poll until the pid is gone" loop (Stop's exit-confirmation, F5) hangs until it
// times out.
//
// The child is deliberately NOT reaped (no cmd.Wait) until the assertion is done,
// so this exercises exactly that state. processAlive must report it dead.
func TestProcessAliveZombieChildIsNotAlive(t *testing.T) {
	// `true` exits 0 immediately. Invoked directly (no shell) so there is no
	// command-interpretation layer at all.
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}

	pid := cmd.Process.Pid

	// Reap only after the assertion, so the child stays a zombie throughout it.
	t.Cleanup(func() { _ = cmd.Wait() })

	deadline := time.Now().Add(5 * time.Second)

	for {
		if !processAlive(pid) {
			return // correct: the exited-but-unreaped child reports dead
		}

		if time.Now().After(deadline) {
			t.Fatalf("pid %d still reports alive after it exited: "+
				"processAlive is fooled by an unreaped zombie child", pid)
		}

		time.Sleep(20 * time.Millisecond)
	}
}

// TestProcessAliveLiveChildIsAlive is the counterpart: a child that is genuinely
// still running must report alive, so the zombie fix cannot be satisfied by simply
// always returning false.
func TestProcessAliveLiveChildIsAlive(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}

	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	if !processAlive(cmd.Process.Pid) {
		t.Fatalf("a running child (pid %d) must report alive", cmd.Process.Pid)
	}
}
