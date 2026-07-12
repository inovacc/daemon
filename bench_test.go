package daemon

import (
	"testing"
	"time"
)

func BenchmarkRestartGuardIsLoop(b *testing.B) {
	g := newRestartGuard(defaultGuardSize, defaultGuardWindow)
	now := time.Now()

	i := 0
	for b.Loop() {
		g.isLoop(now.Add(time.Duration(i) * time.Millisecond))
		i++
	}
}

func BenchmarkRestartGuardBackoff(b *testing.B) {
	g := newRestartGuard(defaultGuardSize, defaultGuardWindow)

	i := 0
	for b.Loop() {
		_ = g.backoff(i % 12)
		i++
	}
}

func BenchmarkBuildWorkerArgs(b *testing.B) {
	o := Options{BinaryName: "app", HTTPPort: 8080, GRPCPort: 8081}.withDefaults()

	for b.Loop() {
		_ = o.buildWorkerArgs()
	}
}
