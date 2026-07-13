//go:build !windows

package daemon

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
)

// prepareGracefulShutdown (F4) returns a cmd.Cancel-compatible func that sends
// SIGTERM to the worker, and a no-op cleanup (Unix needs no handle teardown).
// SIGTERM is caught by RunWorker's signal.NotifyContext(ctx, os.Interrupt,
// syscall.SIGTERM), so the worker unwinds via ordinary context cancellation
// instead of dying immediately.
func prepareGracefulShutdown(cmd *exec.Cmd) (cancel func() error, cleanup func()) {
	return func() error {
		if cmd.Process == nil {
			return fmt.Errorf("daemon: signal worker: process is nil")
		}

		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("daemon: signal worker pid %d: %w", cmd.Process.Pid, err)
		}

		return nil
	}, func() {}
}

// watchGracefulShutdown is a Windows-only concern (named-event based, see
// worker_signal_windows.go); on Unix, SIGTERM is already handled by RunWorker's
// signal.NotifyContext, so this is a no-op passthrough.
func watchGracefulShutdown(ctx context.Context) (context.Context, func()) {
	return ctx, func() {}
}
