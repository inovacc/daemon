package main

import (
	"fmt"

	appsvc "github.com/inovacc/daemon/internal/service"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

type serviceProgram struct{}

func (p *serviceProgram) Start(s service.Service) error {
	// The generated internal/service.Handler has a Cobra-style signature and
	// is not context-cancellable, so it is launched in its own goroutine.
	go func() { _ = appsvc.Handler(nil, nil) }()
	return nil
}

func (p *serviceProgram) Stop(s service.Service) error {
	// TODO: internal/service.Handler exposes no cancellation hook; graceful
	// shutdown requires the handler to accept a context. Returning nil lets
	// the service manager terminate the process.
	return nil
}

func newService() (service.Service, error) {
	cfg := &service.Config{
		Name:        "daemon",
		DisplayName: "daemon",
		Description: "daemon service",
		Arguments:   []string{"service", "run"},
	}
	return service.New(&serviceProgram{}, cfg)
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the daemon OS service",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install daemon as an OS service",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := newService()
		if err != nil {
			return fmt.Errorf("service install: %w", err)
		}
		if err := s.Install(); err != nil {
			return fmt.Errorf("service install (needs admin/root?): %w", err)
		}
		fmt.Println("installed")
		return nil
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the daemon OS service",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := newService()
		if err != nil {
			return fmt.Errorf("service uninstall: %w", err)
		}
		if err := s.Uninstall(); err != nil {
			return fmt.Errorf("service uninstall: %w", err)
		}
		fmt.Println("uninstalled")
		return nil
	},
}

var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon service",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := newService()
		if err != nil {
			return fmt.Errorf("service start: %w", err)
		}
		if err := s.Start(); err != nil {
			return fmt.Errorf("service start: %w", err)
		}
		fmt.Println("started")
		return nil
	},
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon service",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := newService()
		if err != nil {
			return fmt.Errorf("service stop: %w", err)
		}
		if err := s.Stop(); err != nil {
			return fmt.Errorf("service stop: %w", err)
		}
		fmt.Println("stopped")
		return nil
	},
}

var serviceRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the daemon service",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := newService()
		if err != nil {
			return fmt.Errorf("service restart: %w", err)
		}
		if err := s.Restart(); err != nil {
			return fmt.Errorf("service restart: %w", err)
		}
		fmt.Println("restarted")
		return nil
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := newService()
		if err != nil {
			return fmt.Errorf("service status: %w", err)
		}
		st, err := s.Status()
		if err != nil {
			fmt.Println("not installed")
			return nil
		}
		switch st {
		case service.StatusRunning:
			fmt.Println("running")
		case service.StatusStopped:
			fmt.Println("stopped")
		default:
			fmt.Println("not installed")
		}
		return nil
	},
}

var serviceRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run daemon as a service (invoked by the OS)",
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := newService()
		if err != nil {
			return fmt.Errorf("service run: %w", err)
		}
		return s.Run()
	},
}

func init() {
	serviceCmd.AddCommand(
		serviceInstallCmd,
		serviceUninstallCmd,
		serviceStartCmd,
		serviceStopCmd,
		serviceRestartCmd,
		serviceStatusCmd,
		serviceRunCmd,
	)
	rootCmd.AddCommand(serviceCmd)
}
