//go:build windows

package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// gracefulShutdownEnv is the env var the monitor uses to tell the worker the name
// of the Windows event object it should wait on for a graceful-shutdown request.
// Read by watchGracefulShutdown in the worker process.
const gracefulShutdownEnv = "DAEMON_GRACEFUL_SHUTDOWN_EVENT"

var gracefulEventSeq atomic.Uint64

// eventSecurityAttributes builds a SECURITY_ATTRIBUTES whose PROTECTED DACL grants
// access ONLY to the creating user, LocalSystem, and the Administrators group.
//
// This matters: the graceful-shutdown event is a NAMED kernel object, and a named
// object created with the DEFAULT security descriptor can be opened by other
// processes in the same session. Without this, any local process could open
// `Local\daemon-graceful-*` and SetEvent it, triggering an unauthenticated
// graceful shutdown of the daemon — a local denial of service. The creator's own
// SID is always granted, so the monitor→worker path (same user) is never locked
// out. Mirrors the sibling `indexer` project's named-pipe DACL hardening.
func eventSecurityAttributes() (*windows.SecurityAttributes, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("token user: %w", err)
	}

	// D:P = protected DACL (drops inherited ACEs). GA = GENERIC_ALL.
	// SY = LocalSystem, BA = BUILTIN\Administrators, plus the creator's own SID.
	sddl := "D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;" + user.User.Sid.String() + ")"

	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, fmt.Errorf("security descriptor: %w", err)
	}

	return &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}, nil
}

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
//
// The event is created with a restrictive DACL (eventSecurityAttributes) so that
// only the same user, SYSTEM, and Administrators can open and signal it.
func prepareGracefulShutdown(cmd *exec.Cmd) (cancel func() error, cleanup func()) {
	name := fmt.Sprintf(`Local\daemon-graceful-%d-%d-%d`,
		os.Getpid(), time.Now().UnixNano(), gracefulEventSeq.Add(1))

	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return func() error { return fmt.Errorf("daemon: graceful shutdown event name: %w", err) }, func() {}
	}

	// Restrict the event to the creating user + SYSTEM + Administrators. A failure
	// to build the descriptor is not fatal: fall back to the default DACL (which is
	// no LESS restrictive than the pre-hardening behavior) rather than lose graceful
	// shutdown entirely — but say so loudly.
	sa, saErr := eventSecurityAttributes()
	if saErr != nil {
		slog.Warn("daemon: could not build a restrictive DACL for the graceful-shutdown event; using the default",
			slog.Any("err", saErr))
	}

	// manualReset=1 (must be explicitly Reset, never auto-clears): a manual-reset
	// event guarantees the worker observes the signal even if it hasn't started
	// waiting on it yet at the moment SetEvent is called. initialState=0 (unsignaled).
	handle, err := windows.CreateEvent(sa, 1, 0, namePtr)
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
//
// The waiting goroutine blocks on TWO objects: the graceful event AND a local
// "done" event that the returned cleanup signals. Without the done event the wait
// would be an uncancellable WaitForSingleObject(INFINITE), so a worker that exits
// for ANY other reason (normal return, its own signal) would have cleanup call
// CloseHandle while that wait was still outstanding — closing a handle with a
// pending wait is documented-unsafe on Windows (handle-reuse hazard). Signalling
// done first, then joining the goroutine, guarantees the wait has returned before
// any handle is closed.
func watchGracefulShutdown(ctx context.Context) (context.Context, func()) {
	name := os.Getenv(gracefulShutdownEnv)
	if name == "" {
		return ctx, func() {}
	}

	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return ctx, func() {}
	}

	// SYNCHRONIZE is all that is needed to wait on the event. The restrictive DACL
	// the monitor set grants the same user GENERIC_ALL, which covers it.
	event, err := windows.OpenEvent(windows.SYNCHRONIZE, false, namePtr)
	if err != nil {
		return ctx, func() {}
	}

	// Local, unnamed manual-reset event, used only to break the wait below. Unnamed
	// => not reachable by any other process, so it needs no DACL of its own.
	done, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		_ = windows.CloseHandle(event)

		return ctx, func() {}
	}

	watchCtx, cancel := context.WithCancel(ctx)
	waitDone := make(chan struct{})

	go func() {
		defer close(waitDone)

		// Returns as soon as EITHER the monitor signals the graceful event or
		// cleanup signals done — so no wait is ever outstanding at close time.
		ev, waitErr := windows.WaitForMultipleObjects([]windows.Handle{event, done}, false, windows.INFINITE)
		if waitErr == nil && ev == windows.WAIT_OBJECT_0 {
			// Index 0 == the graceful event: the monitor asked us to stop.
			cancel()
		}
	}()

	return watchCtx, func() {
		// Order matters: signal done -> join the waiter -> only then close handles.
		_ = windows.SetEvent(done)

		<-waitDone

		cancel()

		_ = windows.CloseHandle(event)
		_ = windows.CloseHandle(done)
	}
}
