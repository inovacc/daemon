package daemon_test

import (
	"context"
	"fmt"
	"sort"

	"github.com/inovacc/daemon"
	"github.com/spf13/cobra"
)

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
