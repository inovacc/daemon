package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/inovacc/daemon/internal/parameters"
	"github.com/inovacc/daemon/pkg/daemon"

	"github.com/inovacc/config"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "daemon",
	Short: "daemon is a CLI application",
	Long: `daemon is a CLI application

This is a CLI application built with Cobra.`,
}

// serve is the demo worker body: it blocks until the context is cancelled so the
// monitor→worker supervision chain (and the OS service via svc run) has something
// real to run.
func serve(ctx context.Context, p daemon.Ports) error {
	slog.Info("serving", slog.Int("http_port", p.HTTP), slog.Int("grpc_port", p.GRPC))
	<-ctx.Done()
	slog.Info("stopped serving")
	return nil
}

// Execute runs the root command and maps any error to a process exit code via
// daemon.ExitCodeFor (so svc privilege failures from C4 surface as exit 5 rather
// than a generic 1).
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(daemon.ExitCodeFor(err))
	}
}

func main() {
	Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.Version = GetVersionJSON()
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml", "config file (default is config.yaml)")

	if err := daemon.AttachCommands(rootCmd, daemon.Options{
		BinaryName: "daemon",
		Serve:      serve,
	}); err != nil {
		cobra.CheckErr(err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile == "" {
		_, _ = fmt.Fprint(os.Stdout, "Using default config file: config.yaml")
	}

	// Load configuration from a file, applying defaults if needed
	if err := config.InitServiceConfig(&parameters.Service{}, cfgFile); err != nil {
		_, _ = fmt.Fprint(os.Stdout, "failed to load config: %w", err)
	}
}
