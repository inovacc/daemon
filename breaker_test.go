package daemon

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock is a monotonic, advance-on-demand clock used by breaker tests.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Unix(0, 0).UTC()}
}

func (f *fakeClock) now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.t
}

func (f *fakeClock) advance(d time.Duration) {
	f.mu.Lock()
	f.t = f.t.Add(d)
	f.mu.Unlock()
}

func testBreakerCfg() BreakerConfig {
	return BreakerConfig{MaxRestarts: 5, Window: 60 * time.Second}
}

func TestBreaker_InitialState(t *testing.T) {
	b := NewBreaker(testBreakerCfg())
	if got := b.State(); got != BreakerClosed {
		t.Fatalf("initial state = %v, want CLOSED", got)
	}

	if !b.ShouldAllowRestart() {
		t.Fatal("fresh breaker must allow restarts")
	}
}

func TestBreaker_BelowThresholdStaysClosed(t *testing.T) {
	fc := newFakeClock()
	b := NewBreakerWithClock(testBreakerCfg(), fc.now)

	// 4 restarts within 60s → still CLOSED, 5th attempt allowed.
	for i := range 4 {
		st := b.RecordRestart()
		if st != BreakerClosed {
			t.Fatalf("after %d restarts: state = %v, want CLOSED", i+1, st)
		}

		fc.advance(5 * time.Second)
	}

	if !b.ShouldAllowRestart() {
		t.Fatal("5th attempt must be allowed (only 4 events in window)")
	}
}

func TestBreaker_TripsAtThreshold(t *testing.T) {
	fc := newFakeClock()
	b := NewBreakerWithClock(testBreakerCfg(), fc.now)

	for i := range 5 {
		fc.advance(1 * time.Second)

		st := b.RecordRestart()
		if i < 4 && st != BreakerClosed {
			t.Fatalf("event %d: state = %v, want CLOSED", i+1, st)
		}

		if i == 4 && st != BreakerOpenTerminal {
			t.Fatalf("event %d: state = %v, want OPEN_TERMINAL", i+1, st)
		}
	}

	if b.ShouldAllowRestart() {
		t.Fatal("OPEN_TERMINAL breaker must refuse restarts")
	}
}

func TestBreaker_SlidingWindowPrunes(t *testing.T) {
	fc := newFakeClock()
	b := NewBreakerWithClock(testBreakerCfg(), fc.now)

	// 5 restarts spaced 20s apart: at the 5th Record, events older than
	// (now-60s) are pruned, leaving only 4 in-window entries → still CLOSED.
	for range 5 {
		fc.advance(20 * time.Second)
		b.RecordRestart()
	}

	if got := b.State(); got != BreakerClosed {
		t.Fatalf("after 5 spaced records: state = %v, want CLOSED (window should prune oldest)", got)
	}
}

func TestBreaker_OpenTerminalIsSticky(t *testing.T) {
	fc := newFakeClock()
	b := NewBreakerWithClock(testBreakerCfg(), fc.now)

	for range 5 {
		fc.advance(1 * time.Second)
		b.RecordRestart()
	}

	if b.State() != BreakerOpenTerminal {
		t.Fatal("expected OPEN_TERMINAL after 5 rapid restarts")
	}
	// Advance far beyond window; state must persist.
	fc.advance(10 * time.Minute)

	if b.State() != BreakerOpenTerminal {
		t.Fatal("OPEN_TERMINAL must persist even after window elapses")
	}

	if b.ShouldAllowRestart() {
		t.Fatal("OPEN_TERMINAL must never allow restarts")
	}
	// Recording more events is a no-op (already terminal).
	st := b.RecordRestart()
	if st != BreakerOpenTerminal {
		t.Fatalf("RecordRestart on OPEN_TERMINAL: got %v", st)
	}
}

func TestBreaker_ConcurrentRecord(t *testing.T) {
	fc := newFakeClock()
	// MaxRestarts set higher than total events so state stays CLOSED throughout
	// and every RecordRestart increments the counter (once OPEN_TERMINAL, Record
	// becomes a no-op by design).
	b := NewBreakerWithClock(BreakerConfig{MaxRestarts: 10_000, Window: 60 * time.Second}, fc.now)

	const (
		goroutines = 50
		perG       = 10
	)

	var (
		wg      sync.WaitGroup
		started atomic.Int32
	)

	gate := make(chan struct{})

	for range goroutines {
		wg.Go(func() {
			started.Add(1)
			<-gate

			for range perG {
				b.RecordRestart()
			}
		})
	}

	for started.Load() < goroutines {
		time.Sleep(time.Millisecond)
	}

	close(gate)
	wg.Wait()

	if got := b.EventCount(); got != goroutines*perG {
		t.Fatalf("event count = %d, want %d", got, goroutines*perG)
	}
	// MaxRestarts=10000, total=500 → still CLOSED, but every event counted.
	if b.State() != BreakerClosed {
		t.Fatalf("state = %v, want CLOSED (race test: verifying all increments landed)", b.State())
	}
}

func TestBreaker_BreakerStateString(t *testing.T) {
	if BreakerClosed.String() != "CLOSED" {
		t.Errorf("CLOSED.String() = %q", BreakerClosed.String())
	}

	if BreakerOpenTerminal.String() != "OPEN_TERMINAL" {
		t.Errorf("OPEN_TERMINAL.String() = %q", BreakerOpenTerminal.String())
	}

	if BreakerState(99).String() != "unknown" {
		t.Errorf("unknown.String() = %q", BreakerState(99).String())
	}
}

func TestBreaker_Reset(t *testing.T) {
	fc := newFakeClock()

	b := NewBreakerWithClock(testBreakerCfg(), fc.now)
	for range 5 {
		fc.advance(time.Second)
		b.RecordRestart()
	}

	if b.State() != BreakerOpenTerminal {
		t.Fatal("expected OPEN_TERMINAL before reset")
	}

	b.Reset()

	if b.State() != BreakerClosed {
		t.Fatalf("state after Reset = %v, want CLOSED", b.State())
	}

	if b.EventCount() != 0 {
		t.Fatalf("EventCount after Reset = %d, want 0", b.EventCount())
	}
}

func TestBreaker_EventCountPrunes(t *testing.T) {
	fc := newFakeClock()
	b := NewBreakerWithClock(testBreakerCfg(), fc.now)
	b.RecordRestart()
	b.RecordRestart()

	if b.EventCount() != 2 {
		t.Fatalf("EventCount = %d, want 2", b.EventCount())
	}

	fc.advance(61 * time.Second)

	if got := b.EventCount(); got != 0 {
		t.Fatalf("EventCount after window elapses = %d, want 0", got)
	}
}

func TestBreaker_NilClockDefaults(t *testing.T) {
	b := NewBreakerWithClock(testBreakerCfg(), nil)
	// Should not panic; basic operation works.
	if b.State() != BreakerClosed {
		t.Fatal("expected CLOSED")
	}

	b.RecordRestart()

	if b.EventCount() != 1 {
		t.Fatal("expected 1 event")
	}
}
