package daemon

import "time"

// restartGuard protects against spawn/fork loop hell: a worker that crashes and is
// restarted unboundedly. It combines a sliding-window rate limiter (abort after N
// restarts inside a threshold window) with exponential backoff between restarts.
//
// Note: ExitRestart (intentional restart) must bypass this guard entirely — the
// monitor loop `continue`s without calling isLoop or backoff for code 3.
//
// Concurrency: restartGuard is NOT safe for concurrent use. It is owned and mutated
// by a single goroutine — the monitor loop — and must not be shared across goroutines
// without external synchronization.
type restartGuard struct {
	size   int           // max restarts allowed within the window (e.g. 4)
	window time.Duration // threshold window (e.g. 60s)
	times  []time.Time   // timestamps of recent restarts, oldest first
}

func newRestartGuard(size int, window time.Duration) *restartGuard {
	return &restartGuard{size: size, window: window}
}

// isLoop records a restart at now and reports whether the last `size` restarts all
// fall within `window` — i.e. the worker is crash-looping and the monitor should abort.
func (g *restartGuard) isLoop(now time.Time) bool {
	// Drop timestamps that have aged out of the window, compacting in place so the
	// backing array is reused.
	cutoff := now.Add(-g.window)

	kept := 0

	for _, t := range g.times {
		if t.After(cutoff) {
			g.times[kept] = t
			kept++
		}
	}

	g.times = append(g.times[:kept], now)

	return len(g.times) >= g.size
}

// backoff returns the delay before the given restart attempt: min(1s*2^attempt, window-cap).
func (g *restartGuard) backoff(attempt int) time.Duration {
	const maxDelay = 60 * time.Second

	d := time.Second
	for range attempt {
		d *= 2
		if d >= maxDelay {
			return maxDelay
		}
	}

	if d > maxDelay {
		return maxDelay
	}

	return d
}
