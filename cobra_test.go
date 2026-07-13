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

// F2: `service <bogus>` must NOT silently start the daemon (RunMonitor). Without
// Args: cobra.NoArgs, Cobra swallows the unrecognized positional arg and RunE still
// fires. This pins the fix by asserting the monitor is never invoked and an error
// is returned.
func TestServiceCommandRejectsUnknownArgs(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	monitorInvoked := false

	if err := AttachCommands(root, Options{
		BinaryName: "t",
		DataDir:    t.TempDir(),
		Serve: func(context.Context, Ports) error {
			monitorInvoked = true
			return nil
		},
	}); err != nil {
		t.Fatalf("AttachCommands: %v", err)
	}

	root.SetArgs([]string{"service", "bogus"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))

	err := root.Execute()
	if err == nil {
		t.Fatal("`service bogus` must return an error, not silently succeed")
	}

	if monitorInvoked {
		t.Fatal("`service bogus` must NOT invoke RunMonitor (Options.Serve)")
	}
}

// F8: IsSupervisorCommand identifies the hidden __monitor/__worker commands (and
// only those), so a consumer with its own exit-code contract can route their
// errors through daemon.ExitCodeFor while routing everything else through its own
// mapper.
func TestIsSupervisorCommand(t *testing.T) {
	root := &cobra.Command{Use: "root"}

	o := Options{
		BinaryName: "t",
		DataDir:    t.TempDir(),
		Serve:      func(context.Context, Ports) error { return nil },
	}.withDefaults()

	if err := AttachCommands(root, o); err != nil {
		t.Fatalf("AttachCommands: %v", err)
	}

	find := func(name string) *cobra.Command {
		cmd, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatalf("Find(%q): %v", name, err)
		}

		return cmd
	}

	if !IsSupervisorCommand(find("__monitor"), o) {
		t.Error("__monitor must be identified as a supervisor command")
	}

	if !IsSupervisorCommand(find("__worker"), o) {
		t.Error("__worker must be identified as a supervisor command")
	}

	if IsSupervisorCommand(find("service"), o) {
		t.Error("service must NOT be identified as a supervisor command")
	}

	if IsSupervisorCommand(nil, o) {
		t.Error("IsSupervisorCommand(nil, o) must be false")
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

// F7: `service start` against an already-running daemon still prints the friendly
// "already running" text, but now returns ErrAlreadyRunning (-> exit 6 via
// ExitCodeFor) instead of nil, so callers can script on the state. This is an
// intentional 0.2.0 breaking behavior change (see CHANGELOG.md).
func TestStartCommandAlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	// A live record (this test's own pid) makes Start return ErrAlreadyRunning.
	_ = serverinfo.NewStore(dir).Write(serverinfo.Info{PID: os.Getpid()})

	out, err := runSubcommand(t, startCommand(Options{BinaryName: "t", DataDir: dir}.withDefaults()), "start")
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("already-running must return ErrAlreadyRunning, got %v", err)
	}

	if got := ExitCodeFor(err); got != int(ExitAlreadyRunning) {
		t.Fatalf("ExitCodeFor(err) = %d, want %d (ExitAlreadyRunning)", got, ExitAlreadyRunning)
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

// F7: `service stop` against an idle daemon still prints the friendly "not
// running" text, but now returns ErrNotRunning (-> exit 7 via ExitCodeFor) instead
// of nil, so callers can script on the state. This is an intentional 0.2.0
// breaking behavior change (see CHANGELOG.md).
func TestStopCommandIdempotentWhenNotRunning(t *testing.T) {
	// No record → Stop returns ErrNotRunning; the command reports it as benign
	// ("not running") but still surfaces the sentinel via the returned error.
	out, err := runSubcommand(t, stopCommand(Options{BinaryName: "t", DataDir: t.TempDir()}.withDefaults()), "stop")
	if !errors.Is(err, ErrNotRunning) {
		t.Fatalf("stopping a stopped daemon must return ErrNotRunning, got %v", err)
	}

	if got := ExitCodeFor(err); got != int(ExitNotRunning) {
		t.Fatalf("ExitCodeFor(err) = %d, want %d (ExitNotRunning)", got, ExitNotRunning)
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

// F7: `service status` against an idle daemon still prints "not running", but now
// returns ErrNotRunning (-> exit 7 via ExitCodeFor) instead of nil — mirroring the
// systemd convention that status on an inactive unit is non-zero. Intentional
// 0.2.0 breaking behavior change (see CHANGELOG.md).
func TestStatusCommandNotRunning(t *testing.T) {
	out, err := runSubcommand(t, statusCommand(Options{BinaryName: "t", DataDir: t.TempDir()}.withDefaults()), "status")
	if !errors.Is(err, ErrNotRunning) {
		t.Fatalf("status on an idle daemon must return ErrNotRunning, got %v", err)
	}

	if got := ExitCodeFor(err); got != int(ExitNotRunning) {
		t.Fatalf("ExitCodeFor(err) = %d, want %d (ExitNotRunning)", got, ExitNotRunning)
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
