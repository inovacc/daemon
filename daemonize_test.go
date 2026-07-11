package daemon

import (
	"errors"
	"os"
	"testing"

	"github.com/inovacc/daemon/internal/serverinfo"
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
