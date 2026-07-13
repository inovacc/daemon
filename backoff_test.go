package daemon

import (
	"math/rand"
	"testing"
	"time"
)

func TestBackoff_Next_ExponentialGrowthWithCap(t *testing.T) {
	b := Backoff{
		Base:       1 * time.Second,
		Cap:        60 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.25,
	}
	r := rand.New(rand.NewSource(1))

	// For attempt N, pre-jitter = min(Base*2^N, Cap).
	// Post-jitter must be within [pre*(1-J), pre*(1+J)].
	for attempt := 0; attempt <= 10; attempt++ {
		got := b.Next(attempt, r)
		rawNano := float64(b.Base) * pow2(attempt)

		capped := rawNano
		if capped > float64(b.Cap) {
			capped = float64(b.Cap)
		}

		lo := time.Duration(capped * 0.75)
		hi := time.Duration(capped * 1.25)
		// Also respect absolute clamps
		absLo := time.Duration(float64(b.Base) * 0.75)
		absHi := time.Duration(float64(b.Cap) * 1.25)

		if lo < absLo {
			lo = absLo
		}

		if hi > absHi {
			hi = absHi
		}

		if got < lo || got > hi {
			t.Errorf("attempt=%d: got %v, want in [%v, %v] (capped=%v)", attempt, got, lo, hi, time.Duration(capped))
		}
	}
}

func TestBackoff_Next_Deterministic(t *testing.T) {
	b := Backoff{Base: time.Second, Cap: 60 * time.Second, Multiplier: 2.0, Jitter: 0.25}
	r1 := rand.New(rand.NewSource(1))
	r2 := rand.New(rand.NewSource(1))

	for attempt := range 5 {
		got1 := b.Next(attempt, r1)

		got2 := b.Next(attempt, r2)
		if got1 != got2 {
			t.Fatalf("attempt=%d: not deterministic: %v vs %v", attempt, got1, got2)
		}
	}
}

func TestBackoff_Next_CappedAtMaxAttempt(t *testing.T) {
	b := Backoff{Base: time.Second, Cap: 60 * time.Second, Multiplier: 2.0, Jitter: 0.25}
	r := rand.New(rand.NewSource(42))
	// attempt=100 would be astronomical without cap
	got := b.Next(100, r)
	// must be <= Cap*(1+J)
	maxAllowed := time.Duration(float64(b.Cap) * 1.25)
	if got > maxAllowed {
		t.Fatalf("attempt=100: got %v, want <= %v", got, maxAllowed)
	}
	// must be >= Cap*(1-J) (pre-jitter is capped at Cap)
	minAllowed := time.Duration(float64(b.Cap) * 0.75)
	if got < minAllowed {
		t.Fatalf("attempt=100: got %v, want >= %v", got, minAllowed)
	}
}

func TestBackoff_Next_ZeroJitter(t *testing.T) {
	b := Backoff{Base: time.Second, Cap: 60 * time.Second, Multiplier: 2.0, Jitter: 0.0}
	r := rand.New(rand.NewSource(1))

	got := b.Next(2, r) // 1s * 4 = 4s exactly
	if got != 4*time.Second {
		t.Fatalf("got %v, want 4s exactly with zero jitter", got)
	}
}

func TestBackoff_Next_NegativeAttemptTreatedAsZero(t *testing.T) {
	b := Backoff{Base: time.Second, Cap: 60 * time.Second, Multiplier: 2.0, Jitter: 0.0}
	r := rand.New(rand.NewSource(1))

	got := b.Next(-5, r)
	if got != time.Second {
		t.Fatalf("got %v, want 1s (negative attempt should act like 0)", got)
	}
}

func TestBackoff_Next_NilRandOK(t *testing.T) {
	b := Backoff{Base: time.Second, Cap: 60 * time.Second, Multiplier: 2.0, Jitter: 0.25}
	// should not panic; value is bounded
	got := b.Next(1, nil)
	if got <= 0 {
		t.Fatalf("nil rand produced non-positive delay: %v", got)
	}
}

func TestFromConfig(t *testing.T) {
	cfg := BackoffConfig{Base: 2 * time.Second, Cap: 30 * time.Second, Multiplier: 1.5, Jitter: 0.1}

	b := FromConfig(cfg)
	if b.Base != cfg.Base || b.Cap != cfg.Cap || b.Multiplier != cfg.Multiplier || b.Jitter != cfg.Jitter {
		t.Fatalf("FromConfig mismatch: %+v vs %+v", b, cfg)
	}
}

// pow2 computes 2^n using integer shift for small n, float Pow otherwise.
func pow2(n int) float64 {
	x := 1.0
	for range n {
		x *= 2.0
	}

	return x
}

func TestRestartGuardBackoffMatchesLegacyFormula(t *testing.T) {
	// The legacy restartGuard.backoff formula was: d=1s; double per attempt;
	// cap at 60s. Backoff with Base=1s/Cap=60s/Multiplier=2/Jitter=0 must
	// reproduce it bit-for-bit — this is the back-compat contract for
	// Options.Backoff == nil.
	b := Backoff{Base: time.Second, Cap: 60 * time.Second, Multiplier: 2.0, Jitter: 0}

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
		if got := b.Next(c.attempt, nil); got != c.want {
			t.Fatalf("Next(%d)=%v want %v", c.attempt, got, c.want)
		}
	}
}
