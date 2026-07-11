package daemon

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// autostartMethod identifies a Windows launch-at-logon mechanism. The two methods
// mirror the two approaches Syncthing documents for Windows, and together with the
// `svc` group they reflect how Google ships its own software: a privileged
// (LocalSystem) service PLUS logon/boot triggers.
//
//   - methodStartup       registry Run key (HKCU per-user / HKLM all-users)
//     https://docs.syncthing.net/users/autostart.html#autostart-windows-startup
//   - methodTaskScheduler a Task Scheduler ONLOGON task, optionally elevated
//     https://docs.syncthing.net/users/autostart.html#autostart-windows-taskschd
//     https://learn.microsoft.com/windows/win32/TaskSchd/task-scheduler-start-page
type autostartMethod string

const (
	methodStartup       autostartMethod = "startup"
	methodTaskScheduler autostartMethod = "taskscheduler"
)

// autostartVerb is the subcommand a STANDALONE autostart entry launches:
// "<exe> service" (the monitor supervisor). The combined `svc install
// --autostart` path instead launches "svc start" so the trigger merely asks the
// SCM to start the already-installed service (Google-style: service + trigger,
// one process). Kept in one place so the Run-key value and the scheduled-task
// action stay identical for a given mode.
const autostartVerb = "service"

// defaultLaunchArgs is what a standalone autostart entry runs.
var defaultLaunchArgs = []string{autostartVerb}

// autostartEntry is a single reported autostart registration.
type autostartEntry struct {
	Method  autostartMethod
	Scope   string // "user", "machine", or "logon"
	Enabled bool
	Target  string // the launched command line, when known
}

// autostartManager is the platform seam behind the `autostart` verbs. Windows
// implements it against the registry Run key + schtasks.exe; other platforms
// return a friendly not-supported error. Tests inject a fake via
// newAutostartManager.
type autostartManager interface {
	Enable(method autostartMethod, elevated bool) error
	Disable(method autostartMethod, elevated bool) error
	Status() ([]autostartEntry, error)
}

// newAutostartManager is the construction seam (realAutostartManager per-platform
// in production). launch is the argv the registered entry runs (e.g.
// {"service"} standalone, or {"svc","start"} for the combined svc-install
// trigger). Tests override it to inject a fake.
var newAutostartManager = realAutostartManager

// validateServiceName rejects names unsuitable for use as a Windows scheduled-task
// (/TN) name or a registry Run value: empty/whitespace, or any character outside a
// conservative safe set (letters, digits, '-', '_', '.'). Defense-in-depth — the
// exec path already builds arg slices (no shell) and every elevated write is gated
// by RequirePrivilege, but a malformed name should fail fast rather than create a
// surprising or unremovable registration.
func validateServiceName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("daemon: service name must not be empty")
	}

	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			// allowed
		default:
			return fmt.Errorf("daemon: service name %q contains invalid character %q (allowed: letters, digits, - _ .)", name, r)
		}
	}

	return nil
}

// parseMethod validates the --method flag value.
func parseMethod(s string) (autostartMethod, error) {
	switch autostartMethod(strings.ToLower(strings.TrimSpace(s))) {
	case methodStartup:
		return methodStartup, nil
	case methodTaskScheduler:
		return methodTaskScheduler, nil
	default:
		return "", fmt.Errorf("daemon: unknown autostart method %q (want %q or %q)", s, methodStartup, methodTaskScheduler)
	}
}

// autostartCommand builds the `autostart` group: register the daemon to launch at
// logon. Two methods are offered (Syncthing's Startup + Task Scheduler
// approaches); `--elevated` promotes each to an admin/all-users registration
// (HKLM Run key, or a SYSTEM Task Scheduler task with highest privileges), which
// mirrors Google's elevated (LocalSystem) autostart. o is expected to be
// withDefaults()'d.
func autostartCommand(o Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autostart",
		Short: fmt.Sprintf("Manage %s launch-at-logon (Startup / Task Scheduler, Windows)", o.BinaryName),
	}
	cmd.AddCommand(
		autostartEnableCommand(o),
		autostartDisableCommand(o),
		autostartStatusCommand(o),
	)

	return cmd
}

func autostartEnableCommand(o Options) *cobra.Command {
	var (
		method   string
		elevated bool
	)

	c := &cobra.Command{
		Use:   "enable",
		Short: "Register the daemon to start at logon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAutostartMutation(cmd, o, method, elevated, autostartManager.Enable, "enabled")
		},
	}
	c.Flags().StringVar(&method, "method", string(methodStartup), "autostart method: startup|taskscheduler")
	c.Flags().BoolVar(&elevated, "elevated", false, "register all-users/SYSTEM (requires admin)")

	return c
}

func autostartDisableCommand(o Options) *cobra.Command {
	var (
		method   string
		elevated bool
	)

	c := &cobra.Command{
		Use:   "disable",
		Short: "Remove the daemon's logon autostart registration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAutostartMutation(cmd, o, method, elevated, autostartManager.Disable, "disabled")
		},
	}
	c.Flags().StringVar(&method, "method", string(methodStartup), "autostart method: startup|taskscheduler")
	c.Flags().BoolVar(&elevated, "elevated", false, "target the all-users/SYSTEM registration (requires admin)")

	return c
}

// runAutostartMutation is the shared body of enable/disable: parse the method,
// gate the elevated (all-users/SYSTEM) case behind RequirePrivilege exactly like
// the svc verbs, build the manager, then apply op. done is the success word.
func runAutostartMutation(
	cmd *cobra.Command, o Options, method string, elevated bool,
	op func(autostartManager, autostartMethod, bool) error, done string,
) error {
	m, err := parseMethod(method)
	if err != nil {
		return err
	}

	// --elevated writes an all-users / SYSTEM registration, which the OS only
	// permits from an elevated process. Gate it like the svc verbs so the failure
	// mode (and exit code 5) is consistent.
	if elevated {
		if err := RequirePrivilege(cmd); err != nil {
			return err
		}
	}

	mgr, err := newAutostartManager(o, defaultLaunchArgs)
	if err != nil {
		return err
	}

	if err := op(mgr, m, elevated); err != nil {
		return fmt.Errorf("autostart %s: %w", done, err)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), done)

	return nil
}

func autostartStatusCommand(o Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current autostart registrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			mgr, err := newAutostartManager(o, defaultLaunchArgs)
			if err != nil {
				return err
			}

			entries, err := mgr.Status()
			if err != nil {
				return fmt.Errorf("autostart status: %w", err)
			}

			anyEnabled := false

			for _, e := range entries {
				if !e.Enabled {
					continue
				}

				anyEnabled = true

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s (%s): %s\n", e.Method, e.Scope, e.Target)
			}

			if !anyEnabled {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "not enabled")
			}

			return nil
		},
	}
}
