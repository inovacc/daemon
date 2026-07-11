package daemon

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/daemon/internal/serverinfo"
	"github.com/spf13/cobra"
)

func TestChildEnvName(t *testing.T) {
	if got := childEnvName("logger-demo"); got != "LOGGER_DEMO_DAEMON_CHILD" {
		t.Errorf("childEnvName = %q", got)
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
