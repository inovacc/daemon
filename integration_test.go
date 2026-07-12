package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Integration-test env contract. When DAEMON_IT_COUNTER is set, this binary was
// spawned by a monitor under test as the WORKER: it bumps a shared counter file and
// crashes for the first DAEMON_IT_CRASHES spawns, then exits cleanly so the monitor
// stops. TestMain intercepts this BEFORE the test framework parses flags, so the
// __worker cobra args the monitor passes are harmless.
const (
	itCounterEnv       = "DAEMON_IT_COUNTER"
	itCrashesEnv       = "DAEMON_IT_CRASHES"
	itSuiteHardTimeout = 120 * time.Second
)

func itBumpCounter(path string) int {
	b, _ := os.ReadFile(path)
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	n++
	_ = os.WriteFile(path, []byte(strconv.Itoa(n)), 0o600)

	return n
}

func itReadCounter(path string) int {
	b, _ := os.ReadFile(path)
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))

	return n
}

func TestMain(m *testing.M) {
	// Spawned worker role: play it and exit; never recurse into the suite.
	if path := os.Getenv(itCounterEnv); path != "" {
		n := itBumpCounter(path)
		crashN, _ := strconv.Atoi(os.Getenv(itCrashesEnv))

		if n <= crashN {
			os.Exit(ExitError.AsInt()) // crash
		}

		os.Exit(ExitSuccess.AsInt()) // clean exit -> monitor returns
	}

	// Hard timeout: a hung supervisor must fail the suite fast, not wedge CI forever.
	done := make(chan int, 1)

	go func() { done <- m.Run() }()

	select {
	case code := <-done:
		os.Exit(code)
	case <-time.After(itSuiteHardTimeout):
		_, _ = fmt.Fprintln(os.Stderr, "daemon: test suite hard timeout exceeded")

		os.Exit(1)
	}
}

// TestIntegrationMonitorRestartsCrashingWorker drives the REAL spawn path (realSpawn,
// not a seam): the monitor spawns this test binary as a worker that crashes twice then
// exits clean, and must restart it each time and return nil once it exits cleanly.
func TestIntegrationMonitorRestartsCrashingWorker(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns real worker processes; skipped under -short")
	}

	dir := t.TempDir()
	counter := filepath.Join(dir, "count")

	t.Setenv(itCounterEnv, counter) // inherited by the spawned worker
	t.Setenv(itCrashesEnv, "2")

	o := Options{BinaryName: "daemon-it", DataDir: dir}.withDefaults()
	m := newMonitor(o)               // real realSpawn
	m.sleep = func(time.Duration) {} // skip backoff so the test is fast

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := m.run(ctx); err != nil {
		t.Fatalf("monitor should return nil after the worker finally exits clean, got %v", err)
	}

	if got := itReadCounter(counter); got != 3 {
		t.Fatalf("worker spawned %d times, want 3 (2 crashes + 1 clean exit)", got)
	}
}
