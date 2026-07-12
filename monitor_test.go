package daemon

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/inovacc/daemon/internal/serverinfo"
)

func newMonitorForTest(t *testing.T, spawn func(context.Context, []string) int) *monitor {
	t.Helper()
	o := Options{BinaryName: "t", DataDir: t.TempDir()}.withDefaults()
	m := newMonitor(o)
	m.spawn = spawn
	m.sleep = func(time.Duration) {} // no real waiting in tests

	return m
}

// TestRunMonitorReturnsOnCancelledContext exercises the public RunMonitor entry
// point (not the internal m.run seam): with an already-cancelled context the loop
// stops at its top-of-loop guard before spawning any worker, and the deferred
// cleanup removes the server.json it wrote on startup.
func TestRunMonitorReturnsOnCancelledContext(t *testing.T) {
	dir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel: the loop must return before the first spawn.

	if err := RunMonitor(ctx, Options{BinaryName: "t", DataDir: dir}); err != nil {
		t.Fatalf("cancelled context should return nil, got %v", err)
	}
	// The monitor writes server.json on startup and removes it via defer on exit.
	if serverinfo.NewStore(dir).IsRunning() != nil {
		t.Fatal("server.json must be removed after the monitor stops")
	}
}

// TestMonitorStopsWhenContextCancelledDuringCrash covers handleCrash's ctx.Done()
// branch: a worker crash whose context is cancelled mid-handling stops the monitor
// cleanly (return nil) instead of restarting or tripping the loop guard.
func TestMonitorStopsWhenContextCancelledDuringCrash(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	m := newMonitorForTest(t, func(context.Context, []string) int {
		calls++

		cancel() // cancel synchronously so handleCrash sees ctx.Done() ready.

		return ExitError.AsInt()
	})

	if err := m.run(ctx); err != nil {
		t.Fatalf("cancellation during crash handling must return nil, got %v", err)
	}

	if calls != 1 {
		t.Fatalf("worker should spawn once then stop on cancellation, got %d", calls)
	}
}

func TestMonitorOnRestartHookFires(t *testing.T) {
	var (
		calls      int
		lastCode   ExitStatus
		lastAttempt int
	)

	o := Options{BinaryName: "t", DataDir: t.TempDir()}.withDefaults()
	o.OnRestart = func(code ExitStatus, attempt int) {
		calls++
		lastCode = code
		lastAttempt = attempt
	}

	m := newMonitor(o)
	m.sleep = func(time.Duration) {}

	const crashes = 2

	n := 0
	m.spawn = func(context.Context, []string) int {
		n++
		if n <= crashes {
			return ExitError.AsInt() // crash twice, then exit clean
		}

		return ExitSuccess.AsInt()
	}

	if err := m.run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}

	if calls != crashes {
		t.Fatalf("OnRestart fired %d times, want %d", calls, crashes)
	}

	if lastCode != ExitError {
		t.Fatalf("OnRestart code = %v, want ExitError", lastCode)
	}

	if lastAttempt != crashes {
		t.Fatalf("OnRestart last attempt = %d, want %d", lastAttempt, crashes)
	}
}

func TestMonitorAbortsOnCrashLoop(t *testing.T) {
	calls := 0
	m := newMonitorForTest(t, func(context.Context, []string) int {
		calls++
		return ExitError.AsInt() // crash every time
	})

	err := m.run(context.Background())
	if err == nil {
		t.Fatal("monitor must abort (return error) on a crash loop")
	}

	if calls != m.o.GuardSize {
		t.Fatalf("worker should be spawned exactly GuardSize=%d times before abort, got %d", m.o.GuardSize, calls)
	}
}

func TestMonitorStopsOnCleanExit(t *testing.T) {
	calls := 0

	m := newMonitorForTest(t, func(context.Context, []string) int {
		calls++
		return ExitSuccess.AsInt()
	})
	if err := m.run(context.Background()); err != nil {
		t.Fatalf("clean worker exit should return nil, got %v", err)
	}

	if calls != 1 {
		t.Fatalf("clean exit should spawn worker once, got %d", calls)
	}
}

func TestRestartExitBypassesGuard(t *testing.T) {
	// More ExitRestart than GuardSize must NOT trip the loop guard.
	calls := 0

	m := newMonitorForTest(t, func(context.Context, []string) int {
		calls++
		if calls <= defaultGuardSize+2 {
			return ExitRestart.AsInt()
		}

		return ExitSuccess.AsInt()
	})
	if err := m.run(context.Background()); err != nil {
		t.Fatalf("intentional restarts must not trip the guard, got %v", err)
	}

	if calls != defaultGuardSize+3 {
		t.Fatalf("expected %d spawns, got %d", defaultGuardSize+3, calls)
	}
}

func TestUpgradeInvokesReexec(t *testing.T) {
	orig := reexecFn

	t.Cleanup(func() { reexecFn = orig })

	var gotArgs []string

	called := false
	reexecFn = func(args []string) error {
		called = true
		gotArgs = args

		return nil // success path returns nil here (real impl never returns)
	}

	// reexecFn returns nil (stubbed success) so the loop continues; the toggling
	// closure requests upgrade on the first spawn, then exits clean on the next.
	first := true
	m := newMonitorForTest(t, func(context.Context, []string) int {
		if first {
			first = false
			return ExitUpgrade.AsInt()
		}

		return ExitSuccess.AsInt()
	})

	if err := m.run(context.Background()); err != nil {
		t.Fatalf("run should return nil, got %v", err)
	}

	if !called {
		t.Fatal("ExitUpgrade must invoke reexecFn")
	}
	// The re-exec reuses the process's ORIGINAL invocation args (os.Args[1:]) so the
	// re-execed image keeps its role (__monitor stays __monitor), not buildMonitorArgs().
	want := os.Args[1:]
	if len(gotArgs) != len(want) {
		t.Fatalf("reexecFn called with %v, want %v", gotArgs, want)
	}

	for i := range want {
		if gotArgs[i] != want[i] {
			t.Fatalf("reexecFn arg %d = %q, want %q", i, gotArgs[i], want[i])
		}
	}
}

func TestUpgradeReexecErrorDegradesToRestart(t *testing.T) {
	orig := reexecFn

	t.Cleanup(func() { reexecFn = orig })

	reexecFn = func([]string) error {
		return errors.New("boom") // re-exec failed; monitor must NOT die
	}

	calls := 0
	m := newMonitorForTest(t, func(context.Context, []string) int {
		calls++
		if calls == 1 {
			return ExitUpgrade.AsInt() // first: request upgrade (reexec fails)
		}

		return ExitSuccess.AsInt() // then: restart spawns worker, exits clean
	})

	if err := m.run(context.Background()); err != nil {
		t.Fatalf("a failed re-exec must degrade to restart, not error: %v", err)
	}

	if calls != 2 {
		t.Fatalf("expected restart after failed re-exec (2 spawns), got %d", calls)
	}
}

// TestUpgradePersistentReexecFailureTripsGuard pins the busy-loop fix: a worker that
// keeps requesting an upgrade whose re-exec always fails must be treated as a crash
// loop — backing off and tripping the fork-loop guard after GuardSize spawns — rather
// than looping forever with attempt reset to 0 and no delay.
func TestUpgradePersistentReexecFailureTripsGuard(t *testing.T) {
	orig := reexecFn

	t.Cleanup(func() { reexecFn = orig })

	reexecFn = func([]string) error { return errors.New("boom") }

	calls := 0
	m := newMonitorForTest(t, func(context.Context, []string) int {
		calls++

		return ExitUpgrade.AsInt() // always upgrade; reexec always fails
	})

	err := m.run(context.Background())
	if err == nil {
		t.Fatal("a persistently failing upgrade must trip the guard, not busy-loop forever")
	}

	if calls != m.o.GuardSize {
		t.Fatalf("worker should spawn exactly GuardSize=%d times before abort, got %d", m.o.GuardSize, calls)
	}
}
