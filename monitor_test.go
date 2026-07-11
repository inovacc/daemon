package daemon

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func newMonitorForTest(t *testing.T, spawn func(context.Context, []string) int) *monitor {
	t.Helper()
	o := Options{BinaryName: "t", DataDir: t.TempDir()}.withDefaults()
	m := newMonitor(o)
	m.spawn = spawn
	m.sleep = func(time.Duration) {} // no real waiting in tests

	return m
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
