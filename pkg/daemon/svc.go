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
	return &cobra.Command{
		Use:   "install",
		Short: "Register the service with the OS init system (privileged)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := newOSService(o)
			if err != nil {
				return err
			}
			if err := s.Install(); err != nil {
				return fmt.Errorf("svc install: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "installed")
			return nil
		},
	}
}

func svcUninstallCommand(o Options) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the service from the OS init system (privileged)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := newOSService(o)
			if err != nil {
				return err
			}
			if err := s.Uninstall(); err != nil {
				return fmt.Errorf("svc uninstall: %w", err)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "uninstalled")
			return nil
		},
	}
}

func svcStartCommand(o Options) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Ask the OS init system to start the service (privileged)",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			st, err := s.Status()
			if err != nil {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "not installed")
				return nil
			}
			switch st {
			case service.StatusRunning:
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "running")
			case service.StatusStopped:
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "stopped")
			default:
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "unknown")
			}
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
