package daemon

import (
	"sync"
	"time"
)

// BreakerState is the state of a Breaker.
//
// Only two states exist — there is no half-open / auto-reset. A daemon
// process that has crash-looped hard enough to trip the breaker is
// considered unrecoverable for the remainder of its lifetime; the operator
// (or a supervising OS service manager) is expected to intervene, not the
// process itself.
type BreakerState int

const (
	// BreakerClosed is the normal operating state: restarts are allowed as
	// long as the sliding-window event count stays below MaxRestarts.
	BreakerClosed BreakerState = iota
	// BreakerOpenTerminal is the terminal state: the breaker has observed
	// MaxRestarts events within Window and refuses all further restarts for
	// the lifetime of the process. It is in-memory only — a fresh process
	// (e.g. after a manual restart of the monitor itself) starts with a
	// fresh, CLOSED breaker.
	BreakerOpenTerminal
)

// String returns the stable external name of the state (safe to use in log
// records and structured event payloads).
func (s BreakerState) String() string {
	switch s {
	case BreakerClosed:
		return "CLOSED"
	case BreakerOpenTerminal:
		return "OPEN_TERMINAL"
	default:
		return "unknown"
	}
}

// BreakerConfig tunes a Breaker's sliding-window trip point.
//
// MaxRestarts is the number of events allowed inside Window before the
// breaker trips to BreakerOpenTerminal. Both fields must be > 0 for the
// breaker to do anything useful; NewBreaker does not validate them — the
// caller (typically Options.withDefaults / resolveBreakerConfig) is
// responsible for filling in sane defaults.
type BreakerConfig struct {
	MaxRestarts int
	Window      time.Duration
}

// Breaker is a sliding-window circuit breaker for a crash/restart loop.
//
// It tracks restart events inside a trailing time window. Once the number of
// events recorded inside that window reaches MaxRestarts, the breaker
// transitions to BreakerOpenTerminal and never returns to BreakerClosed
// during the process's lifetime — there is deliberately no half-open /
// auto-reset behavior, because a process that is crash-looping fast enough
// to trip the breaker should not silently keep retrying forever.
//
// Breaker is safe for concurrent use.
type Breaker struct {
	maxRestarts int
	window      time.Duration
	clock       func() time.Time

	mu     sync.Mutex
	events []time.Time
	state  BreakerState
}

// NewBreaker constructs a breaker from cfg, using wall-clock time.
func NewBreaker(cfg BreakerConfig) *Breaker {
	return NewBreakerWithClock(cfg, time.Now)
}

// NewBreakerWithClock constructs a breaker with an injected clock — used by
// tests (and by restartGuard, which drives the breaker with an explicit
// per-call timestamp) to get deterministic sliding-window behavior. A nil
// clock falls back to time.Now.
func NewBreakerWithClock(cfg BreakerConfig, clock func() time.Time) *Breaker {
	if clock == nil {
		clock = time.Now
	}

	return &Breaker{
		maxRestarts: cfg.MaxRestarts,
		window:      cfg.Window,
		clock:       clock,
		state:       BreakerClosed,
	}
}

// ShouldAllowRestart reports whether the caller may proceed with another
// restart attempt. It prunes events older than the sliding window first.
// Once the state is BreakerOpenTerminal it always returns false.
func (b *Breaker) ShouldAllowRestart() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == BreakerOpenTerminal {
		return false
	}

	b.pruneLocked()

	return len(b.events) < b.maxRestarts
}

// RecordRestart records a restart attempt at the current clock time, prunes
// stale events, and transitions the breaker to BreakerOpenTerminal if the
// sliding window now contains at least MaxRestarts events. The new state is
// returned. Calling RecordRestart on an already-terminal breaker is a no-op
// that returns BreakerOpenTerminal.
func (b *Breaker) RecordRestart() BreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == BreakerOpenTerminal {
		return b.state
	}

	b.events = append(b.events, b.clock())
	b.pruneLocked()

	if len(b.events) >= b.maxRestarts {
		b.state = BreakerOpenTerminal
	}

	return b.state
}

// State returns the current breaker state.
func (b *Breaker) State() BreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.state
}

// EventCount returns the number of events currently inside the window after
// pruning. Useful for picking a backoff attempt counter (attempt N = the Nth
// historical event, before recording the current one).
func (b *Breaker) EventCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.pruneLocked()

	return len(b.events)
}

// Reset clears all recorded events and returns the breaker to BreakerClosed.
// Intended for tests and for callers that manage their own recovery policy
// (e.g. an operator-triggered "clear the breaker" admin action); production
// restart-loop handling never needs to call this — "restart the process =
// clean breaker" is the normal recovery path.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.events = nil
	b.state = BreakerClosed
}

// pruneLocked removes events older than now-window from the front of the
// events slice. Called with b.mu held.
func (b *Breaker) pruneLocked() {
	if len(b.events) == 0 {
		return
	}

	cutoff := b.clock().Add(-b.window)

	i := 0
	for i < len(b.events) && b.events[i].Before(cutoff) {
		i++
	}

	if i > 0 {
		b.events = b.events[i:]
	}
}
