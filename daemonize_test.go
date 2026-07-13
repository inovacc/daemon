package daemon

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/daemon/internal/serverinfo"
	"github.com/spf13/cobra"
)

func TestChildEnvName(t *testing.T) {
	got := childEnvName("logger-demo")
	// Readable sanitized prefix + a hash suffix.
	if !strings.HasPrefix(got, "LOGGER_DEMO_DAEMON_CHILD_") {
		t.Errorf("childEnvName = %q, want the sanitized prefix", got)
	}
	// Deterministic: same input → same var.
	if got != childEnvName("logger-demo") {
		t.Error("childEnvName must be deterministic")
	}
	// Collision-free: names that sanitize identically must still differ.
	if childEnvName("my-app") == childEnvName("my_app") {
		t.Error("distinct binary names must not share a recursion-guard var")
	}
}

func TestStartRefusesFromChild(t *testing.T) {
	o := Options{BinaryName: "t", DataDir: t.TempDir()}
	t.Setenv(childEnvName("t"), "1")

	if _, err := Start(o); err == nil {
		t.Error("Start must refuse from within a daemon child")
	}
}

func TestStartAlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	// Write a serverinfo whose PID is THIS test process (guaranteed alive).
	_ = serverinfo.NewStore(dir).Write(serverinfo.Info{PID: os.Getpid()})

	pid, err := Start(Options{BinaryName: "t", DataDir: dir})
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("err = %v, want ErrAlreadyRunning", err)
	}

	if pid != os.Getpid() {
		t.Errorf("pid = %d, want %d", pid, os.Getpid())
	}
}

func TestStartSpawnsAndWaits(t *testing.T) {
	dir := t.TempDir()
	store := serverinfo.NewStore(dir)
	orig := spawnDetachedFn

	t.Cleanup(func() { spawnDetachedFn = orig })
	// Fake spawn: simulate the monitor by writing serverinfo, return a fake pid.
	spawnDetachedFn = func(exe string, args, env []string) (int, error) {
		// the guard env must be present in the child env
		found := false

		for _, e := range env {
			if e == childEnvName("t")+"=1" {
				found = true
			}
		}

		if !found {
			t.Error("child env guard not set on spawn")
		}

		_ = store.Write(serverinfo.Info{PID: os.Getpid()})

		return 4242, nil
	}

	pid, err := Start(Options{BinaryName: "t", DataDir: dir})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if pid != 4242 {
		t.Errorf("pid = %d, want 4242", pid)
	}
}

func TestStartHealthCheckTimeout(t *testing.T) {
	dir := t.TempDir()
	origSpawn, origTimeout := spawnDetachedFn, healthWaitTimeout

	t.Cleanup(func() { spawnDetachedFn, healthWaitTimeout = origSpawn, origTimeout })

	healthWaitTimeout = 20 * time.Millisecond
	// Spawn succeeds but the "monitor" never writes serverinfo (e.g. it crashed on
	// startup). Start must surface the unconfirmed state, not report silent success.
	spawnDetachedFn = func(_ string, _, _ []string) (int, error) { return 4242, nil }

	pid, err := Start(Options{BinaryName: "t", DataDir: dir})
	if !errors.Is(err, ErrHealthCheckTimeout) {
		t.Fatalf("err = %v, want ErrHealthCheckTimeout", err)
	}
	// The spawned pid must still come back so the caller can inspect/kill it.
	if pid != 4242 {
		t.Errorf("pid = %d, want 4242", pid)
	}
}

func TestStartCommandReportsUnconfirmed(t *testing.T) {
	dir := t.TempDir()
	origSpawn, origTimeout := spawnDetachedFn, healthWaitTimeout

	t.Cleanup(func() { spawnDetachedFn, healthWaitTimeout = origSpawn, origTimeout })

	healthWaitTimeout = 20 * time.Millisecond
	spawnDetachedFn = func(_ string, _, _ []string) (int, error) { return 4242, nil }

	root := &cobra.Command{Use: "daemon"}
	root.AddCommand(startCommand(Options{BinaryName: "t", DataDir: dir}.withDefaults()))

	var buf bytes.Buffer

	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"start"})

	if err := root.Execute(); err != nil {
		t.Fatalf("start: %v", err) // unconfirmed is not a hard CLI failure
	}

	out := buf.String()
	if !strings.Contains(out, "4242") || !strings.Contains(out, "unconfirmed") {
		t.Fatalf("output %q must report the pid and an unconfirmed status", out)
	}
}

// F3: Status reports running=true and the recorded PID for a live instance.
func TestStatusRunning(t *testing.T) {
	dir := t.TempDir()
	_ = serverinfo.NewStore(dir).Write(serverinfo.Info{PID: os.Getpid()})

	running, pid, err := Status(Options{BinaryName: "t", DataDir: dir})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if !running {
		t.Fatal("Status should report running=true for a live pid")
	}

	if pid != os.Getpid() {
		t.Errorf("pid = %d, want %d", pid, os.Getpid())
	}
}

// F3: Status reports running=false (nil error) when no serverinfo exists.
func TestStatusNotRunningNoRecord(t *testing.T) {
	running, pid, err := Status(Options{BinaryName: "t", DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if running {
		t.Fatal("Status should report running=false when there is no record")
	}

	if pid != 0 {
		t.Errorf("pid = %d, want 0", pid)
	}
}

// F3: Status reports running=false (nil error), not an error, for a stale record
// whose PID is dead — mirroring serverinfo.Store.IsRunning's self-healing.
func TestStatusNotRunningDeadPID(t *testing.T) {
	dir := t.TempDir()
	// A PID essentially guaranteed to be dead / not-ours in this test environment:
	// spawn and immediately reap a short-lived process, then reuse its now-dead pid.
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatalf("spawn short-lived helper: %v", err)
	}

	deadPID := cmd.ProcessState.Pid()

	_ = serverinfo.NewStore(dir).Write(serverinfo.Info{PID: deadPID})

	running, pid, err := Status(Options{BinaryName: "t", DataDir: dir})
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if running {
		t.Fatalf("Status should report running=false for a dead pid %d", deadPID)
	}

	if pid != 0 {
		t.Errorf("pid = %d, want 0", pid)
	}

	// The stale record should have been self-healed (removed) as a side effect.
	if serverinfo.NewStore(dir).IsRunning() != nil {
		t.Error("stale serverinfo should have been removed")
	}
}

func TestStopNotRunning(t *testing.T) {
	if err := Stop(Options{BinaryName: "t", DataDir: t.TempDir()}); !errors.Is(err, ErrNotRunning) {
		t.Errorf("err = %v, want ErrNotRunning", err)
	}
}

func TestStopCallsPlatformStop(t *testing.T) {
	dir := t.TempDir()
	_ = serverinfo.NewStore(dir).Write(serverinfo.Info{PID: os.Getpid()})
	orig := stopProcessFn

	t.Cleanup(func() { stopProcessFn = orig })

	called := 0
	stopProcessFn = func(pid int) error { called = pid; return nil }

	if err := Stop(Options{BinaryName: "t", DataDir: dir}); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if called != os.Getpid() {
		t.Errorf("stopProcess called with %d, want %d", called, os.Getpid())
	}
	// serverinfo should be removed after stop
	if serverinfo.NewStore(dir).IsRunning() != nil {
		t.Error("serverinfo not removed after Stop")
	}
}

// TestStopLogsRemoveFailureButSucceeds covers the non-fatal remove path: after the
// worker is killed, a failure to delete server.json must be logged (not swallowed)
// yet Stop must still return nil (the process is already gone; IsRunning self-heals).
func TestStopLogsRemoveFailureButSucceeds(t *testing.T) {
	dir := t.TempDir()
	store := serverinfo.NewStore(dir)
	_ = store.Write(serverinfo.Info{PID: os.Getpid()})

	orig := stopProcessFn

	t.Cleanup(func() { stopProcessFn = orig })
	// IsRunning reads the valid file first; then the "kill" swaps server.json for a
	// NON-EMPTY directory, which os.Remove refuses on every platform — forcing the
	// warning branch deterministically without a Remove seam.
	stopProcessFn = func(int) error {
		path := store.Path()
		if err := os.Remove(path); err != nil {
			t.Fatalf("setup: remove file: %v", err)
		}

		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatalf("setup: mkdir: %v", err)
		}

		if err := os.WriteFile(filepath.Join(path, "blocker"), []byte("x"), 0o644); err != nil {
			t.Fatalf("setup: write blocker: %v", err)
		}

		return nil
	}

	var buf bytes.Buffer

	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	if err := Stop(Options{BinaryName: "t", DataDir: dir, Logger: logger}); err != nil {
		t.Fatalf("Stop must not fail on a non-fatal remove error, got %v", err)
	}

	if out := buf.String(); !strings.Contains(out, "failed to remove server info file") {
		t.Fatalf("expected a warning about the failed removal, got %q", out)
	}
}

// F6: with GracefulStop set, Stop calls it, then confirms the process is gone via
// Status — NOT via a bare "the hook returned" — before returning. The hook here
// simulates the daemon shutting down cleanly by removing its own serverinfo
// record, which is what a real monitor does on exit. No forced kill (stopProcessFn)
// should be needed.
func TestStopGracefulPathSkipsForceKill(t *testing.T) {
	dir := t.TempDir()
	store := serverinfo.NewStore(dir)
	_ = store.Write(serverinfo.Info{PID: os.Getpid()})

	origPoll := statusPollInterval

	t.Cleanup(func() { statusPollInterval = origPoll })

	statusPollInterval = time.Millisecond

	origStop := stopProcessFn

	t.Cleanup(func() { stopProcessFn = origStop })

	forceKillCalled := false
	stopProcessFn = func(int) error {
		forceKillCalled = true
		return nil
	}

	graceCalled := false
	o := Options{
		BinaryName:  "t",
		DataDir:     dir,
		StopTimeout: 2 * time.Second,
		GracefulStop: func(context.Context) error {
			graceCalled = true
			// Simulate the daemon exiting cleanly on its own.
			return store.Remove()
		},
	}

	if err := Stop(o); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if !graceCalled {
		t.Fatal("GracefulStop was never called")
	}

	if forceKillCalled {
		t.Fatal("a successful graceful stop must not fall back to a forced kill")
	}
}

// F6: when GracefulStop is called but the process is still alive at StopTimeout,
// Stop must fall back to a forced kill (stopProcessFn) instead of reporting a false
// success.
func TestStopGracefulTimeoutFallsBackToForceKill(t *testing.T) {
	dir := t.TempDir()
	store := serverinfo.NewStore(dir)
	_ = store.Write(serverinfo.Info{PID: os.Getpid()}) // this test process: stays "alive"

	origPoll := statusPollInterval

	t.Cleanup(func() { statusPollInterval = origPoll })

	statusPollInterval = time.Millisecond

	origStop := stopProcessFn

	t.Cleanup(func() { stopProcessFn = origStop })

	forceKillCalled := false
	stopProcessFn = func(int) error {
		forceKillCalled = true
		return nil
	}

	o := Options{
		BinaryName:  "t",
		DataDir:     dir,
		StopTimeout: 20 * time.Millisecond,
		GracefulStop: func(context.Context) error {
			return nil // acknowledged, but the (faked) process never actually exits
		},
	}

	if err := Stop(o); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if !forceKillCalled {
		t.Fatal("a graceful-stop timeout must fall back to a forced kill")
	}
}

// F6: a GracefulStop hook that itself errors must still fall back to a forced kill
// rather than leaving the daemon running.
func TestStopGracefulHookErrorFallsBackToForceKill(t *testing.T) {
	dir := t.TempDir()
	_ = serverinfo.NewStore(dir).Write(serverinfo.Info{PID: os.Getpid()})

	origStop := stopProcessFn

	t.Cleanup(func() { stopProcessFn = origStop })

	forceKillCalled := false
	stopProcessFn = func(int) error {
		forceKillCalled = true
		return nil
	}

	o := Options{
		BinaryName:  "t",
		DataDir:     dir,
		StopTimeout: 20 * time.Millisecond,
		GracefulStop: func(context.Context) error {
			return errors.New("ipc unreachable")
		},
	}

	if err := Stop(o); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if !forceKillCalled {
		t.Fatal("a failing GracefulStop hook must fall back to a forced kill")
	}
}

// F6: a nil GracefulStop must behave exactly like pre-0.2.0 Stop — straight to the
// forced kill, no behavior change for existing consumers.
func TestStopNilGracefulStopUsesForceKillDirectly(t *testing.T) {
	dir := t.TempDir()
	_ = serverinfo.NewStore(dir).Write(serverinfo.Info{PID: os.Getpid()})

	origStop := stopProcessFn

	t.Cleanup(func() { stopProcessFn = origStop })

	forceKillCalled := false
	stopProcessFn = func(int) error {
		forceKillCalled = true
		return nil
	}

	if err := Stop(Options{BinaryName: "t", DataDir: dir}); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	if !forceKillCalled {
		t.Fatal("nil GracefulStop must force-kill directly, unchanged from pre-0.2.0 behavior")
	}
}
