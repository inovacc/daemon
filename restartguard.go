package daemon

import "time"

// restartGuard protects against spawn/fork loop hell: a worker that crashes and is
// restarted unboundedly. It combines a sliding-window rate limiter (abort after N
// restarts inside a threshold window) with exponential backoff between restarts.
//
// Implementation note: restartGuard is a thin adapter over the general-purpose
// Breaker (sliding-window trip) and Backoff (jittered exponential delay) types —
// see breaker.go / backoff.go. It exists as its own type (rather than exposing
// Breaker/Backoff directly to the monitor) so the monitor's crash-handling code,
// and the package-private isLoop/backoff call sites and their tests, do not need
// to change shape: newRestartGuard(size, window) still yields the exact legacy
// trip point and the exact legacy deterministic (zero-jitter) backoff curve.
// newRestartGuardFromConfig gives the monitor a way to drive the same adapter
// from a richer BreakerConfig/BackoffConfig (Options.Breaker / Options.Backoff).
//
// Note: ExitRestart (intentional restart) must bypass this guard entirely — the
// monitor loop `continue`s without calling isLoop or backoff for code 3.
//
// Concurrency: restartGuard is NOT safe for concurrent use. It is owned and mutated
// by a single goroutine — the monitor loop — and must not be shared across goroutines
// without external synchronization. (The embedded Breaker is itself safe for
// concurrent use, but restartGuard's own curNow bookkeeping is not.)
type restartGuard struct {
	breaker  *Breaker
	backoffP Backoff // backoff policy; named to avoid shadowing the backoff() method

	// curNow is the timestamp isLoop was last called with. The embedded Breaker
	// is driven by a clock closure that reads this field, so restartGuard.isLoop
	// keeps its legacy signature — isLoop(now time.Time) — where the caller
	// supplies the timestamp explicitly (deterministic tests pass fixed times)
	// instead of the Breaker reading the wall clock itself.
	curNow time.Time

	// times mirrors the length of the breaker's pruned event window after the
	// most recent isLoop call. Kept only for introspection by pre-existing
	// tests (stress_test.go reads len(g.times)); the values themselves carry
	// no meaning beyond count — the Breaker is the source of truth for actual
	// event timestamps.
	times []time.Time
}

// legacyBackoffConfig is the deterministic (zero-jitter) exponential curve the
// original ad-hoc restartGuard.backoff implemented: 1s, 2s, 4s, ... capped at
// 60s. It is still the default whenever a caller does not supply a richer
// BackoffConfig (Options.Backoff == nil), so existing consumers observe
// bit-for-bit identical restart delays after this refactor.
func legacyBackoffConfig() BackoffConfig {
	return BackoffConfig{Base: time.Second, Cap: 60 * time.Second, Multiplier: 2.0, Jitter: 0}
}

// newRestartGuard builds a restartGuard with the legacy deterministic backoff
// curve (see legacyBackoffConfig) and a sliding-window breaker of size/window.
func newRestartGuard(size int, window time.Duration) *restartGuard {
	return newRestartGuardFromConfig(BreakerConfig{MaxRestarts: size, Window: window}, legacyBackoffConfig())
}

// newRestartGuardFromConfig builds a restartGuard from an explicit
// BreakerConfig + BackoffConfig pair, used by newMonitor once Options.Breaker
// / Options.Backoff (or their GuardSize/GuardWindow-derived defaults) have
// been resolved.
func newRestartGuardFromConfig(breakerCfg BreakerConfig, backoffCfg BackoffConfig) *restartGuard {
	g := &restartGuard{backoffP: FromConfig(backoffCfg)}
	g.breaker = NewBreakerWithClock(breakerCfg, func() time.Time { return g.curNow })

	return g
}

// isLoop records a restart at now and reports whether the last `size` restarts all
// fall within `window` — i.e. the worker is crash-looping and the monitor should abort.
func (g *restartGuard) isLoop(now time.Time) bool {
	g.curNow = now

	state := g.breaker.RecordRestart()
	g.times = make([]time.Time, g.breaker.EventCount())

	return state == BreakerOpenTerminal
}

// backoff returns the delay before the given restart attempt: min(1s*2^attempt, window-cap)
// when built via newRestartGuard, or whatever curve the caller supplied via
// newRestartGuardFromConfig.
func (g *restartGuard) backoff(attempt int) time.Duration {
	return g.backoffP.Next(attempt, nil)
}
