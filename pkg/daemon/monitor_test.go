package daemon

import (
	"context"
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
