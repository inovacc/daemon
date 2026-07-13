package daemon

import (
	"context"
	"testing"
	"time"
)

// F4: on context cancellation, realSpawn must give the worker a grace period
// (ShutdownGrace) instead of Go's default immediate Process.Kill(). A worker that
// exits promptly on the graceful stop request — a named Windows event on Windows
// (see prepareGracefulShutdown / watchGracefulShutdown; NOT a CTRL_BREAK console
// event, which cannot reach a CREATE_NO_WINDOW worker), SIGTERM on Unix — must be
// observed to exit cleanly, well within ShutdownGrace, and its real exit code
// (ExitSuccess) must still reach the monitor.
//
// On Windows this is also the end-to-end proof that the event's restrictive DACL
// (creator + SYSTEM + Administrators) still lets the worker — same user, a
// different process — open and wait on the event: if the DACL were too tight, the
// worker's OpenEvent would fail, it would never observe the stop request, and it
// would be force-killed after WaitDelay instead of reporting ExitSuccess.
func TestRealSpawnGracefulWorkerExitsWithoutForceKill(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a real worker process; skipped under -short")
	}

	t.Setenv(itGraceEnv, "graceful")

	o := Options{ShutdownGrace: 5 * time.Second}.withDefaults()

	ctx, cancel := context.WithCancel(context.Background())

	codeCh := make(chan int, 1)

	go func() { codeCh <- realSpawn(ctx, nil, o) }()

	time.Sleep(300 * time.Millisecond) // let the child install its signal handler

	start := time.Now()

	cancel()

	var code int
	select {
	case code = <-codeCh:
	case <-time.After(5 * time.Second):
		t.Fatal("realSpawn did not return within the grace period")
	}

	elapsed := time.Since(start)

	if code != ExitSuccess.AsInt() {
		t.Fatalf("code = %d, want ExitSuccess (%d); the worker's real exit code must survive the graceful signal",
			code, ExitSuccess.AsInt())
	}
	// A graceful exit should be fast, not need to burn through the whole
	// ShutdownGrace window before Go force-kills.
	if elapsed > 3*time.Second {
		t.Fatalf("graceful exit took %s; suspiciously close to ShutdownGrace, force-kill path likely taken", elapsed)
	}
}

// F4: a worker that ignores the graceful signal must be force-killed only after
// WaitDelay (ShutdownGrace) elapses — not immediately, and not never.
func TestRealSpawnStubbornWorkerForceKilledAfterGrace(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a real worker process; skipped under -short")
	}

	t.Setenv(itGraceEnv, "stubborn")

	const grace = 500 * time.Millisecond

	o := Options{ShutdownGrace: grace}.withDefaults()

	ctx, cancel := context.WithCancel(context.Background())

	codeCh := make(chan int, 1)

	go func() { codeCh <- realSpawn(ctx, nil, o) }()

	time.Sleep(300 * time.Millisecond) // let the child install signal.Ignore

	cancel()

	cancelledAt := time.Now()

	var code int
	select {
	case code = <-codeCh:
	case <-time.After(10 * time.Second):
		t.Fatal("realSpawn never returned; the stubborn worker was never force-killed")
	}

	elapsed := time.Since(cancelledAt)

	if code == ExitSuccess.AsInt() {
		t.Fatal("a force-killed worker must not report ExitSuccess")
	}
	// Must have waited roughly the grace period before killing (allow scheduling
	// slack below, but it must not have been near-instant).
	if elapsed < grace/2 {
		t.Fatalf("force-kill fired after only %s, want roughly >= %s (WaitDelay honored)", elapsed, grace)
	}
}
