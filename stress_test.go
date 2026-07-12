package daemon

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// TestMonitorStressNoGoroutineLeak drives many full crash-loop-to-guard-abort cycles
// and asserts the supervisor leaves no goroutines behind — the loop is single-goroutine
// and every spawn is synchronous, so the count must stay flat.
func TestMonitorStressNoGoroutineLeak(t *testing.T) {
	crashAlways := func(context.Context, []string) int { return ExitError.AsInt() }

	// Warm up once so lazily-started runtime goroutines don't skew the baseline.
	_ = newMonitorForTest(t, crashAlways).run(context.Background())

	runtime.GC()

	before := runtime.NumGoroutine()

	for range 50 {
		if err := newMonitorForTest(t, crashAlways).run(context.Background()); err == nil {
			t.Fatal("each run should trip the guard and return an error")
		}
	}

	runtime.GC()

	if after := runtime.NumGoroutine(); after > before+2 {
		t.Fatalf("goroutine leak across 50 supervisor cycles: before=%d after=%d", before, after)
	}
}

// TestRestartGuardStressSlidingWindow hammers the sliding-window compaction: crashes
// spaced beyond the window must never trip the guard, however many occur, and the
// retained-timestamp slice must not grow without bound.
func TestRestartGuardStressSlidingWindow(t *testing.T) {
	g := newRestartGuard(defaultGuardSize, defaultGuardWindow)
	base := time.Now()

	for i := range 1000 {
		// Each crash is one window+1s after the last, so the window only ever holds one.
		at := base.Add(time.Duration(i) * (defaultGuardWindow + time.Second))
		if g.isLoop(at) {
			t.Fatalf("guard falsely tripped on well-spaced crash %d", i)
		}
	}

	if len(g.times) > defaultGuardSize {
		t.Fatalf("retained timestamps grew unbounded: %d", len(g.times))
	}
}
