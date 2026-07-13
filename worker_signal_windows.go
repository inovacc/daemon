//go:build windows

package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows"
)

// gracefulShutdownEnv is the env var the monitor uses to tell the worker the name
// of the Windows event object it should wait on for a graceful-shutdown request.
// Read by watchGracefulShutdown in the worker process.
const gracefulShutdownEnv = "DAEMON_GRACEFUL_SHUTDOWN_EVENT"

var gracefulEventSeq atomic.Uint64

// prepareGracefulShutdown (F4) creates a manual-reset, named Windows event, wires
// its name into the worker's environment, and returns a cmd.Cancel-compatible func
// that signals it, plus a cleanup func the caller must run once the worker has
// exited (releases the monitor's handle).
//
// A named event — not GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, ...) — is used
// deliberately: CTRL events require the sender and receiver to share a console,
// which does not hold in every deployment topology this library supports (a
// worker spawned with CREATE_NO_WINDOW gets its own, separate hidden console; a
// monitor with redirected/absent stdio has none at all). This was confirmed
// empirically: a live spawn+signal probe showed GenerateConsoleCtrlEvent
// returning success while the target process never reacted. A named event has no
// such dependency — it works identically regardless of console state.
func prepareGracefulShutdown(cmd *exec.Cmd) (cancel func() error, cleanup func()) {
	name := fmt.Sprintf(`Local\daemon-graceful-%d-%d-%d`,
		os.Getpid(), time.Now().UnixNano(), gracefulEventSeq.Add(1))

	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return func() error { return fmt.Errorf("daemon: graceful shutdown event name: %w", err) }, func() {}
	}

	// manualReset=1 (must be explicitly Reset, never auto-clears): a manual-reset
	// event guarantees the worker observes the signal even if it hasn't started
	// waiting on it yet at the moment SetEvent is called. initialState=0 (unsignaled).
	handle, err := windows.CreateEvent(nil, 1, 0, namePtr)
	if err != nil {
		return func() error { return fmt.Errorf("daemon: CreateEvent: %w", err) }, func() {}
	}

	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}

	cmd.Env = append(cmd.Env, gracefulShutdownEnv+"="+name)

	return func() error {
			if err := windows.SetEvent(handle); err != nil {
				return fmt.Errorf("daemon: SetEvent(%s): %w", name, err)
			}

			return nil
		}, func() {
			_ = windows.CloseHandle(handle)
		}
}

// watchGracefulShutdown (F4, worker side) opens the Windows event named by
// gracefulShutdownEnv (set by prepareGracefulShutdown in the monitor) and derives
// a child context that is cancelled when the monitor signals it. If the env var is
// unset (e.g. the worker body is being run standalone, not under the monitor) or
// the event cannot be opened, it returns ctx unchanged with a no-op cleanup.
func watchGracefulShutdown(ctx context.Context) (context.Context, func()) {
	name := os.Getenv(gracefulShutdownEnv)
	if name == "" {
		return ctx, func() {}
	}

	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return ctx, func() {}
	}

	handle, err := windows.OpenEvent(windows.SYNCHRONIZE, false, namePtr)
	if err != nil {
		return ctx, func() {}
	}

	watchCtx, cancel := context.WithCancel(ctx)

	go func() {
		_, _ = windows.WaitForSingleObject(handle, windows.INFINITE)

		cancel()
	}()

	return watchCtx, func() {
		cancel()

		_ = windows.CloseHandle(handle)
	}
}
