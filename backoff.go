package daemon

import (
	"math"
	"math/rand"
	"time"
)

// BackoffConfig tunes a Backoff's exponential-with-jitter delay curve.
//
// Base is the pre-jitter delay for attempt 0. Cap is the hard pre-jitter
// upper bound. Multiplier is the exponential growth factor (>= 1.0 — a value
// of 1.0 degenerates to a constant Base delay, still subject to jitter).
// Jitter is the symmetric jitter fraction in [0, 1); 0 means fully
// deterministic delays.
type BackoffConfig struct {
	Base       time.Duration
	Cap        time.Duration
	Multiplier float64
	Jitter     float64
}

// Backoff computes exponentially increasing retry delays with jitter,
// capped at a maximum. Backoff itself is stateless — the caller tracks the
// attempt counter (e.g. the number of restarts already recorded by a
// Breaker) and passes it to Next.
type Backoff BackoffConfig

// FromConfig builds a Backoff from a BackoffConfig.
func FromConfig(cfg BackoffConfig) Backoff {
	return Backoff(cfg)
}

// Next returns the delay to sleep before restart attempt N.
//
// Formula:
//
//	raw        = Base * Multiplier^attempt
//	capped     = min(raw, Cap)
//	jitterFact = 1 + rand.Float64()*2*Jitter - Jitter    // in [1-J, 1+J)
//	delay      = capped * jitterFact
//
// The result is clamped into [Base*(1-Jitter), Cap*(1+Jitter)] to prevent
// underflow on tiny Base values or runaway growth on extreme jitter.
//
// A negative attempt is treated as 0. If r is nil, the package-level
// math/rand global is used.
func (b Backoff) Next(attempt int, r *rand.Rand) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	// raw = Base * Multiplier^attempt — use float math then clamp to Cap.
	rawFloat := float64(b.Base) * math.Pow(b.Multiplier, float64(attempt))

	capped := rawFloat
	if capped > float64(b.Cap) {
		capped = float64(b.Cap)
	}

	var u float64
	if r != nil {
		u = r.Float64()
	} else {
		u = rand.Float64() //nolint:gosec // jitter, not crypto
	}

	jitterFactor := 1.0 + u*2.0*b.Jitter - b.Jitter

	delay := time.Duration(capped * jitterFactor)

	lo := time.Duration(float64(b.Base) * (1.0 - b.Jitter))
	hi := time.Duration(float64(b.Cap) * (1.0 + b.Jitter))

	if delay < lo {
		delay = lo
	}

	if delay > hi {
		delay = hi
	}

	return delay
}
