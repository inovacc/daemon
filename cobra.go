package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inovacc/daemon/internal/serverinfo"
	"github.com/spf13/cobra"
)

// AttachCommands wires the public `service` command and the hidden `__monitor` /
// `__worker` supervisor commands onto root. Options.Serve (the worker body) is required.
//
// Spawn chain: `service` (or `service start`, future) runs the monitor; the monitor
// spawns `__worker --port N --grpc-port N`; the worker runs Options.Serve.
func AttachCommands(root *cobra.Command, opts Options) error {
	if opts.Serve == nil {
		return errors.New("daemon: Options.Serve is required")
	}

	o := opts.withDefaults()
	if err := o.validate(); err != nil {
		return err
	}

	service := &cobra.Command{
		Use:   "service",
		Short: fmt.Sprintf("Run the %s service (monitor supervises the worker)", o.BinaryName),
		RunE:  func(cmd *cobra.Command, _ []string) error { return RunMonitor(cmd.Context(), o) },
	}
	service.AddCommand(statusCommand(o), startCommand(o), stopCommand(o))

	monitor := &cobra.Command{
		Use:    o.MonitorCmd,
		Hidden: true,
		RunE:   func(cmd *cobra.Command, _ []string) error { return RunMonitor(cmd.Context(), o) },
	}

	var httpPort, grpcPort int

	worker := &cobra.Command{
		Use:    o.WorkerCmd,
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			wo := o
			wo.HTTPPort, wo.GRPCPort = httpPort, grpcPort

			return RunWorker(cmd.Context(), wo)
		},
	}
	worker.Flags().IntVar(&httpPort, "port", o.HTTPPort, "HTTP port")
	worker.Flags().IntVar(&grpcPort, "grpc-port", o.GRPCPort, "gRPC port")

	root.AddCommand(service, monitor, worker, svcCommand(o), autostartCommand(o))

	return nil
}

// RunWorker runs the worker body (Options.Serve) with a signal-cancelled context.
func RunWorker(ctx context.Context, opts Options) error {
	o := opts.withDefaults()
	log := o.logger().With(slog.String("role", "worker"))

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("worker serving", slog.Int("http_port", o.HTTPPort), slog.Int("grpc_port", o.GRPCPort))

	err := o.Serve(ctx, Ports{HTTP: o.HTTPPort, GRPC: o.GRPCPort})
	if err != nil {
		log.Error("worker exited with error", slog.Any("err", err))
	} else {
		log.Info("worker exited")
	}

	return err
}

func startCommand(o Options) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the daemon in the background",
		RunE: func(cmd *cobra.Command, _ []string) error {
			pid, err := Start(o)
			if errors.Is(err, ErrAlreadyRunning) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "already running: pid=%d\n", pid)
				return nil
			}

			// Spawned but readiness unconfirmed: report the pid with an explicit
			// unconfirmed status rather than a bare "started" (which would hide a
			// crashed monitor) or a hard error (which would false-alarm a slow one).
			if errors.Is(err, ErrHealthCheckTimeout) {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"started: pid=%d (unconfirmed: monitor did not report ready in time; run 'status' to verify)\n", pid)

				return nil
			}

			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "started: pid=%d\n", pid)

			return nil
		},
	}
}

func stopCommand(o Options) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the background daemon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := Stop(o)
			// Idempotent: stopping something that is already stopped is a benign
			// success, not a failure — mirrors start's ErrAlreadyRunning handling so
			// repeated `stop` (or stop-after-crash) exits 0 instead of erroring.
			if errors.Is(err, ErrNotRunning) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "not running")
				return nil
			}

			if err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "stopped")

			return nil
		},
	}
}

func statusCommand(o Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether the daemon is running",
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := serverinfo.NewStore(o.DataDir).IsRunning()
			if info == nil {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "not running")
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "running: pid=%d addr=%s since=%s\n",
				info.PID, info.Address, info.StartedAt.Format(time.RFC3339))

			return nil
		},
	}
}
