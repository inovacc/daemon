package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

// osService is the subset of kardianos service.Service that the svc verbs use.
// Defining it as an interface lets unit tests inject a fake; the real
// service.Service satisfies it. Constructed via the newOSService seam.
type osService interface {
	Install() error
	Uninstall() error
	Start() error
	Stop() error
	Restart() error
	Status() (service.Status, error)
	Run() error
}

// newOSService is the seam used to build the OS service handle. Tests override it
// to inject a fake; production uses realOSService (kardianos-backed).
var newOSService = realOSService

// program is the kardianos service body. It wraps the daemon supervisor: the OS
// service manager calls Start when the service starts and Stop on shutdown.
type program struct {
	o           Options
	run         func(ctx context.Context, o Options) error // supervisor seam (defaults to RunMonitor)
	stopTimeout time.Duration

	cancel context.CancelFunc
	done   chan struct{}
}

// newProgram builds a program bound to o. o is expected to be withDefaults()'d.
func newProgram(o Options) *program {
	return &program{
		o:           o,
		run:         RunMonitor,
		stopTimeout: 10 * time.Second,
	}
}

// Start launches the supervisor in a cancelable goroutine and returns immediately,
// as required by the kardianos service.Interface contract.
func (p *program) Start(service.Service) error {
	log := p.o.logger().With(slog.String("role", "os-service"))
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})

	go func() {
		defer close(p.done)

		if err := p.run(ctx, p.o); err != nil {
			log.Error("supervisor exited with error", slog.Any("err", err))
		}
	}()

	log.Info("os service started")

	return nil
}

// Stop cancels the supervisor context and waits for it to drain, up to stopTimeout,
// then returns so the OS service manager can terminate the process.
func (p *program) Stop(service.Service) error {
	log := p.o.logger().With(slog.String("role", "os-service"))
	if p.cancel != nil {
		p.cancel()
	}

	if p.done == nil {
		return nil
	}

	select {
	case <-p.done:
		log.Info("os service stopped")
	case <-time.After(p.stopTimeout):
		log.Warn("os service stop timed out; forcing exit", slog.Duration("timeout", p.stopTimeout))
	}

	return nil
}

// realOSService constructs a kardianos service.Service wrapping a program for o.
// It guards the empty ServiceName case with a friendly error before service.New
// (which would otherwise return the opaque service.ErrNameFieldRequired).
func realOSService(o Options) (osService, error) {
	if o.ServiceName == "" {
		return nil, fmt.Errorf("daemon: cannot manage OS service: ServiceName is empty (set Options.BinaryName or Options.ServiceName)")
	}

	cfg := &service.Config{
		Name:        o.ServiceName,
		DisplayName: o.ServiceName,
		Description: fmt.Sprintf("%s service", o.BinaryName),
		Arguments:   []string{"svc", "run"},
	}

	s, err := service.New(newProgram(o), cfg)
	if err != nil {
		return nil, fmt.Errorf("daemon: build OS service: %w", err)
	}

	return s, nil
}

// svcCommand builds the `svc` group: the OS-service (kardianos) lifecycle. o is
// expected to be withDefaults()'d. The mutating verbs (install/uninstall/start/
// stop/restart) begin their RunE with newOSService so C4 can prepend a
// RequirePrivilege(cmd) guard as the first statement.
func svcCommand(o Options) *cobra.Command {
	svc := &cobra.Command{
		Use:   "svc",
		Short: fmt.Sprintf("Manage the %s OS service (install/start/stop/...)", o.BinaryName),
	}
	svc.AddCommand(
		svcInstallCommand(o),
		svcUninstallCommand(o),
		svcStartCommand(o),
		svcStopCommand(o),
		svcRestartCommand(o),
		svcStatusCommand(o),
		svcRunCommand(o),
	)

	return svc
}

func svcInstallCommand(o Options) *cobra.Command {
	var (
		autostart  bool
		methodFlag string
	)

	c := &cobra.Command{
		Use:   "install",
		Short: "Register the service with the OS init system (privileged)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := RequirePrivilege(cmd); err != nil {
				return err
			}

			// Build (and thus validate) the trigger manager BEFORE installing so an
			// unsupported platform or bad --autostart-method fails fast, leaving no
			// half-configured state.
			mgr, method, err := autostartTrigger(o, autostart, methodFlag)
			if err != nil {
				return err
			}

			s, err := newOSService(o)
			if err != nil {
				return err
			}

			if err := s.Install(); err != nil {
				return fmt.Errorf("svc install: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "installed")

			if mgr != nil {
				// svc install already passed RequirePrivilege, so register the
				// elevated (all-users/SYSTEM) trigger — the Google shape: an elevated
				// service PLUS an elevated logon trigger that starts it.
				if err := mgr.Enable(method, true); err != nil {
					return fmt.Errorf("svc install: service installed but autostart failed: %w", err)
				}

				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "autostart enabled")
			}

			return nil
		},
	}
	addAutostartTriggerFlags(c, &autostart, &methodFlag, "also register")

	return c
}

func svcUninstallCommand(o Options) *cobra.Command {
	var (
		autostart  bool
		methodFlag string
	)

	c := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the service from the OS init system (privileged)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := RequirePrivilege(cmd); err != nil {
				return err
			}

			mgr, method, err := autostartTrigger(o, autostart, methodFlag)
			if err != nil {
				return err
			}

			s, err := newOSService(o)
			if err != nil {
				return err
			}

			if err := s.Uninstall(); err != nil {
				return fmt.Errorf("svc uninstall: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "uninstalled")

			if mgr != nil {
				if err := mgr.Disable(method, true); err != nil {
					return fmt.Errorf("svc uninstall: service removed but autostart cleanup failed: %w", err)
				}

				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "autostart disabled")
			}

			return nil
		},
	}
	addAutostartTriggerFlags(c, &autostart, &methodFlag, "also remove")

	return c
}

// autostartTrigger returns the elevated autostart manager + parsed method for the
// combined `svc install/uninstall --autostart` path, or (nil, "", nil) when the
// flag is unset. The trigger launches "svc start" so it merely asks the SCM to
// start the already-installed service (one process, no duplicate supervisor).
func autostartTrigger(o Options, enabled bool, methodFlag string) (autostartManager, autostartMethod, error) {
	if !enabled {
		return nil, "", nil
	}

	method, err := parseMethod(methodFlag)
	if err != nil {
		return nil, "", err
	}

	mgr, err := newAutostartManager(o, []string{"svc", "start"})
	if err != nil {
		return nil, "", err
	}

	return mgr, method, nil
}

// addAutostartTriggerFlags registers the shared --autostart / --autostart-method
// pair on a svc verb. verb is the action word for the help text ("also register"
// / "also remove").
func addAutostartTriggerFlags(c *cobra.Command, autostart *bool, methodFlag *string, verb string) {
	c.Flags().BoolVar(autostart, "autostart", false,
		fmt.Sprintf("%s an elevated logon trigger that starts the service (Windows)", verb))
	c.Flags().StringVar(methodFlag, "autostart-method", string(methodTaskScheduler),
		"trigger mechanism when --autostart: startup|taskscheduler")
}

func svcStartCommand(o Options) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Ask the OS init system to start the service (privileged)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := RequirePrivilege(cmd); err != nil {
				return err
			}

			s, err := newOSService(o)
			if err != nil {
				return err
			}

			if err := s.Start(); err != nil {
				return fmt.Errorf("svc start: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "started")

			return nil
		},
	}
}

func svcStopCommand(o Options) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Ask the OS init system to stop the service (privileged)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := RequirePrivilege(cmd); err != nil {
				return err
			}

			s, err := newOSService(o)
			if err != nil {
				return err
			}

			if err := s.Stop(); err != nil {
				return fmt.Errorf("svc stop: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "stopped")

			return nil
		},
	}
}

func svcRestartCommand(o Options) *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Ask the OS init system to restart the service (privileged)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := RequirePrivilege(cmd); err != nil {
				return err
			}

			s, err := newOSService(o)
			if err != nil {
				return err
			}

			if err := s.Restart(); err != nil {
				return fmt.Errorf("svc restart: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "restarted")

			return nil
		},
	}
}

func svcStatusCommand(o Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Query the OS service status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := newOSService(o)
			if err != nil {
				return err
			}

			// A Status() error means the service is not registered with the
			// init system; that is a normal, friendly outcome (not a failure to
			// propagate). Map both the error case and each status to a label,
			// then print a single result and exit cleanly.
			st, err := s.Status()

			label := "unknown"

			switch {
			case err != nil:
				label = "not installed"
			case st == service.StatusRunning:
				label = "running"
			case st == service.StatusStopped:
				label = "stopped"
			case st == service.StatusUnknown:
				label = "unknown"
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), label)

			return nil
		},
	}
}

func svcRunCommand(o Options) *cobra.Command {
	return &cobra.Command{
		Use:    "run",
		Short:  "Run as an OS service (invoked by the service manager)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := newOSService(o)
			if err != nil {
				return err
			}

			return s.Run()
		},
	}
}
