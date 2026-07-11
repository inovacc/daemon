package daemon

import (
	"testing"
	"time"
)

// base is a fixed reference time so tests are deterministic.
var base = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestNoLoopWhenRestartsAreSlow(t *testing.T) {
	g := newRestartGuard(4, 60*time.Second)
	// 5 restarts spaced 30s apart: never 4 within a 60s window.
	for i := range 5 {
		if g.isLoop(base.Add(time.Duration(i) * 30 * time.Second)) {
			t.Fatalf("restart %d spaced 30s should not trip the guard", i)
		}
	}
}

func TestLoopTripsWhenNRestartsWithinWindow(t *testing.T) {
	g := newRestartGuard(4, 60*time.Second)
	// 4 restarts within ~3s → loop on the 4th.
	got := []bool{
		g.isLoop(base.Add(0 * time.Second)),
		g.isLoop(base.Add(1 * time.Second)),
		g.isLoop(base.Add(2 * time.Second)),
		g.isLoop(base.Add(3 * time.Second)),
	}

	want := []bool{false, false, false, true}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("restart %d: got loop=%v want %v", i, got[i], want[i])
		}
	}
}

func TestWindowSlidesSoOldRestartsDoNotCount(t *testing.T) {
	g := newRestartGuard(4, 60*time.Second)
	g.isLoop(base.Add(0 * time.Second))
	g.isLoop(base.Add(1 * time.Second))
	g.isLoop(base.Add(2 * time.Second))
	// 4th restart 120s later: the first three have aged out of the 60s window.
	if g.isLoop(base.Add(120 * time.Second)) {
		t.Fatal("old restarts outside the window must not trip the guard")
	}
}

func TestBackoffGrowsThenCaps(t *testing.T) {
	g := newRestartGuard(4, 60*time.Second)

	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{6, 60 * time.Second}, // 64s capped to 60s
		{10, 60 * time.Second},
	}
	for _, c := range cases {
		if got := g.backoff(c.attempt); got != c.want {
			t.Fatalf("backoff(%d)=%v want %v", c.attempt, got, c.want)
		}
	}
}
