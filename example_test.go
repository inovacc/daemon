package daemon_test

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/inovacc/daemon"
	"github.com/spf13/cobra"
)

// blockingServe is a minimal worker body: it runs until the supervisor cancels ctx.
func blockingServe(ctx context.Context, _ daemon.Ports) error {
	<-ctx.Done()

	return nil
}

// ExampleStart shows the detached-start flow and how to read its sentinel errors:
// an already-running instance and an unconfirmed (health-timed-out) start are both
// non-fatal, distinct outcomes.
func ExampleStart() {
	pid, err := daemon.Start(daemon.Options{BinaryName: "myapp", Serve: blockingServe})

	switch {
	case errors.Is(err, daemon.ErrAlreadyRunning):
		fmt.Printf("already running: pid=%d\n", pid)
	case errors.Is(err, daemon.ErrHealthCheckTimeout):
		fmt.Printf("started (unconfirmed): pid=%d — run status to verify\n", pid)
	case err != nil:
		fmt.Println("start failed:", err)
	default:
		fmt.Printf("started: pid=%d\n", pid)
	}
}

// ExampleStop shows that stopping an already-stopped daemon is a benign outcome.
func ExampleStop() {
	err := daemon.Stop(daemon.Options{BinaryName: "myapp"})
	if errors.Is(err, daemon.ErrNotRunning) {
		fmt.Println("not running")
		return
	}

	if err != nil {
		fmt.Println("stop failed:", err)
		return
	}

	fmt.Println("stopped")
}

// ExampleRunMonitor runs the supervisor loop in the foreground until the worker exits
// cleanly, the context is cancelled, or the fork-loop guard aborts. It is normally
// invoked by the hidden __monitor command rather than called directly.
func ExampleRunMonitor() {
	if err := daemon.RunMonitor(context.Background(), daemon.Options{
		BinaryName: "myapp",
		Serve:      blockingServe,
	}); err != nil {
		fmt.Println("monitor exited:", err)
	}
}

// ExampleAttachCommands is the minimal consumer wiring: hand AttachCommands your
// Cobra root and a blocking Serve body, and it adds the service / svc / autostart
// command groups that let your app run as a background/OS service. It also serves
// as the reference that used to live in cmd/ before the module was flattened.
func ExampleAttachCommands() {
	root := &cobra.Command{Use: "myapp"}

	err := daemon.AttachCommands(root, daemon.Options{
		BinaryName: "myapp",
		Serve: func(ctx context.Context, _ daemon.Ports) error {
			<-ctx.Done() // block until the supervisor cancels
			return nil
		},
	})
	if err != nil {
		panic(err)
	}

	// Report the visible groups AttachCommands wired onto the root.
	var names []string

	for _, c := range root.Commands() {
		if !c.Hidden {
			names = append(names, c.Name())
		}
	}

	sort.Strings(names)
	fmt.Println(names)
	// Output: [autostart service svc]
}

// ExampleAttachCommands_autostart shows the launch-at-logon (autostart) group that
// AttachCommands wires onto your root: enable / disable / status. Each takes
// --method startup|taskscheduler and --elevated for the all-users/SYSTEM
// registration (Windows). On non-Windows platforms the verbs are no-ops.
func ExampleAttachCommands_autostart() {
	root := &cobra.Command{Use: "myapp"}

	err := daemon.AttachCommands(root, daemon.Options{
		BinaryName: "myapp",
		Serve: func(ctx context.Context, _ daemon.Ports) error {
			<-ctx.Done()
			return nil
		},
	})
	if err != nil {
		panic(err)
	}

	// Locate the autostart group and list the verbs it exposes.
	var autostart *cobra.Command

	for _, c := range root.Commands() {
		if c.Name() == "autostart" {
			autostart = c
		}
	}

	names := make([]string, 0, len(autostart.Commands()))

	for _, c := range autostart.Commands() {
		names = append(names, c.Name())
	}

	sort.Strings(names)
	fmt.Println(names)
	// Output: [disable enable status]
}
