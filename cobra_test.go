package daemon

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/inovacc/daemon/internal/serverinfo"
	"github.com/spf13/cobra"
)

// runSubcommand mounts cmd under a fresh root, runs it with args, and returns the
// combined stdout+stderr plus the execution error. It isolates each command handler
// so its branches can be exercised without a real spawn/OS interaction.
func runSubcommand(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()

	root := &cobra.Command{Use: "daemon"}
	root.AddCommand(cmd)

	var buf bytes.Buffer

	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)

	err := root.Execute()

	return buf.String(), err
}

func TestAttachRequiresServe(t *testing.T) {
	err := AttachCommands(&cobra.Command{Use: "root"}, Options{BinaryName: "t"})
	if err == nil {
		t.Fatal("AttachCommands must reject Options without a Serve func")
	}
}

func TestAttachRegistersServiceAndHiddenCommands(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	err := AttachCommands(root, Options{
		BinaryName: "t",
		Serve:      func(context.Context, Ports) error { return nil },
	})
	if err != nil {
		t.Fatalf("AttachCommands: %v", err)
	}

	want := map[string]bool{"service": false, "__monitor": true, "__worker": true}
	got := map[string]bool{}

	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			got[c.Name()] = c.Hidden
		}
	}

	for name, hidden := range want {
		h, ok := got[name]
		if !ok {
			t.Fatalf("command %q not registered", name)
		}

		if h != hidden {
			t.Fatalf("command %q hidden=%v, want %v", name, h, hidden)
		}
	}
}

func TestRunWorkerInvokesServeWithPorts(t *testing.T) {
	var gotPorts Ports

	sentinel := errors.New("from serve")

	err := RunWorker(context.Background(), Options{
		BinaryName: "t",
		HTTPPort:   7001,
		GRPCPort:   7002,
		Serve: func(_ context.Context, p Ports) error {
			gotPorts = p
			return sentinel
		},
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("RunWorker should return Serve's error, got %v", err)
	}

	if gotPorts.HTTP != 7001 || gotPorts.GRPC != 7002 {
		t.Fatalf("Serve got wrong ports: %+v", gotPorts)
	}
}

func TestAttachRegistersSvcGroup(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	if err := AttachCommands(root, Options{
		BinaryName: "t",
		Serve:      func(context.Context, Ports) error { return nil },
	}); err != nil {
		t.Fatalf("AttachCommands: %v", err)
	}

	var svc *cobra.Command

	for _, c := range root.Commands() {
		if c.Name() == "svc" {
			svc = c
		}
	}

	if svc == nil {
		t.Fatal("svc group not registered by AttachCommands")
	}

	if svc.Hidden {
		t.Fatal("svc group must be visible")
	}

	want := map[string]bool{ // name -> hidden
		"install": false, "uninstall": false, "start": false,
		"stop": false, "restart": false, "status": false, "run": true,
	}

	got := map[string]bool{}
	for _, c := range svc.Commands() {
		got[c.Name()] = c.Hidden
	}

	for name, hidden := range want {
		h, ok := got[name]
		if !ok {
			t.Fatalf("svc subcommand %q not registered", name)
		}

		if h != hidden {
			t.Fatalf("svc %q hidden=%v, want %v", name, h, hidden)
		}
	}
}

func TestStartCommandAlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	// A live record (this test's own pid) makes Start return ErrAlreadyRunning.
	_ = serverinfo.NewStore(dir).Write(serverinfo.Info{PID: os.Getpid()})

	out, err := runSubcommand(t, startCommand(Options{BinaryName: "t", DataDir: dir}.withDefaults()), "start")
	if err != nil {
		t.Fatalf("already-running is not a CLI failure, got err %v", err)
	}

	if !strings.Contains(out, "already running") || !strings.Contains(out, "pid=") {
		t.Fatalf("output %q must report already running with a pid", out)
	}
}

func TestStartCommandSuccess(t *testing.T) {
	dir := t.TempDir()
	store := serverinfo.NewStore(dir)
	orig := spawnDetachedFn

	t.Cleanup(func() { spawnDetachedFn = orig })
	// Fake monitor writes serverinfo immediately, so Start confirms readiness.
	spawnDetachedFn = func(_ string, _, _ []string) (int, error) {
		_ = store.Write(serverinfo.Info{PID: os.Getpid()})
		return 4242, nil
	}

	out, err := runSubcommand(t, startCommand(Options{BinaryName: "t", DataDir: dir}.withDefaults()), "start")
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if !strings.Contains(out, "started: pid=4242") {
		t.Fatalf("output %q must report the confirmed started pid", out)
	}

	if strings.Contains(out, "unconfirmed") {
		t.Fatalf("output %q must not warn unconfirmed on a confirmed start", out)
	}
}

func TestStartCommandPropagatesError(t *testing.T) {
	dir := t.TempDir()
	orig := spawnDetachedFn

	t.Cleanup(func() { spawnDetachedFn = orig })
	// A genuine spawn failure is neither already-running nor unconfirmed: it must
	// surface as a hard CLI error so ExitCodeFor maps it to a non-zero exit.
	spawnDetachedFn = func(_ string, _, _ []string) (int, error) { return 0, errors.New("spawn boom") }

	out, err := runSubcommand(t, startCommand(Options{BinaryName: "t", DataDir: dir}.withDefaults()), "start")
	if err == nil {
		t.Fatalf("a spawn failure must propagate as an error, got output %q", out)
	}

	if !strings.Contains(err.Error(), "spawn boom") {
		t.Fatalf("err = %v, want the underlying spawn failure", err)
	}
}

func TestStopCommandSuccess(t *testing.T) {
	dir := t.TempDir()
	_ = serverinfo.NewStore(dir).Write(serverinfo.Info{PID: os.Getpid()})
	orig := stopProcessFn

	t.Cleanup(func() { stopProcessFn = orig })

	stopProcessFn = func(int) error { return nil }

	out, err := runSubcommand(t, stopCommand(Options{BinaryName: "t", DataDir: dir}.withDefaults()), "stop")
	if err != nil {
		t.Fatalf("stop: %v", err)
	}

	if !strings.Contains(out, "stopped") {
		t.Fatalf("output %q must confirm the daemon stopped", out)
	}
}

func TestStopCommandIdempotentWhenNotRunning(t *testing.T) {
	// No record → Stop returns ErrNotRunning; the command must treat that as a
	// benign, exit-0 outcome ("not running"), not a hard CLI failure.
	out, err := runSubcommand(t, stopCommand(Options{BinaryName: "t", DataDir: t.TempDir()}.withDefaults()), "stop")
	if err != nil {
		t.Fatalf("stopping a stopped daemon must not error, got %v", err)
	}

	if !strings.Contains(out, "not running") {
		t.Fatalf("output %q must report not running", out)
	}

	if strings.Contains(out, "stopped") {
		t.Fatalf("output %q must not claim stopped when nothing was running", out)
	}
}

func TestStopCommandPropagatesError(t *testing.T) {
	dir := t.TempDir()
	_ = serverinfo.NewStore(dir).Write(serverinfo.Info{PID: os.Getpid()})
	orig := stopProcessFn

	t.Cleanup(func() { stopProcessFn = orig })
	// A real termination failure (not ErrNotRunning) must still propagate so the
	// idempotency shortcut cannot mask a genuine stop error.
	sentinel := errors.New("kill boom")
	stopProcessFn = func(int) error { return sentinel }

	out, err := runSubcommand(t, stopCommand(Options{BinaryName: "t", DataDir: dir}.withDefaults()), "stop")
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want the underlying stop failure", err)
	}

	if strings.Contains(out, "stopped") {
		t.Fatalf("output %q must not claim stopped on a failed stop", out)
	}
}

func TestStatusCommandNotRunning(t *testing.T) {
	out, err := runSubcommand(t, statusCommand(Options{BinaryName: "t", DataDir: t.TempDir()}.withDefaults()), "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if !strings.Contains(out, "not running") {
		t.Fatalf("output %q must report not running", out)
	}
}

func TestStatusCommandRunning(t *testing.T) {
	dir := t.TempDir()
	_ = serverinfo.NewStore(dir).Write(serverinfo.Info{PID: os.Getpid(), Address: "127.0.0.1:9500"})

	out, err := runSubcommand(t, statusCommand(Options{BinaryName: "t", DataDir: dir}.withDefaults()), "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	if !strings.Contains(out, "running: pid=") || !strings.Contains(out, "127.0.0.1:9500") {
		t.Fatalf("output %q must report the running pid and address", out)
	}
}
