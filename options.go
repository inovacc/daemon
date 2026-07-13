package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Default ports for the worker. Always passed to the worker child so the role and
// ports are visible in process listings (ps / Task Manager).
const (
	DefaultHTTPPort = 9500
	DefaultGRPCPort = 9501
)

const (
	defaultGuardSize   = 4
	defaultGuardWindow = 60 * time.Second

	// defaultShutdownGrace is how long the monitor waits for the worker to exit on
	// its own after asking it to stop (prepareGracefulShutdown), before Go
	// force-kills it via cmd.WaitDelay.
	defaultShutdownGrace = 20 * time.Second

	// defaultStopTimeout is how long Stop waits for the process to actually exit
	// after GracefulStop before falling back to a forced kill.
	defaultStopTimeout = 30 * time.Second
)

// Ports is the resolved port pair handed to the worker body.
type Ports struct {
	HTTP int
	GRPC int
}

// Options configures the daemon layer. Consumers fill this and call AttachCommands.
type Options struct {
	// BinaryName is the program name (used for data dir + help). Required.
	BinaryName string
	// ServiceName is the OS-service registration name. Defaults to BinaryName.
	ServiceName string
	// DataDir holds server.json, lock, logs. Defaults to <UserCacheDir>/<BinaryName>.
	DataDir string
	// Version is reported in server.json and `service status`.
	Version string

	HTTPPort int
	GRPCPort int
	// portsExplicit marks that the user overrode the ports (so the monitor forwards them).
	portsExplicit bool

	// GuardSize / GuardWindow tune the fork-loop guard (abort after N restarts in window).
	GuardSize   int
	GuardWindow time.Duration

	// ShutdownGrace is how long the monitor waits for the worker to exit on its own
	// after asking it to stop, before force-killing it. This applies whenever the
	// monitor's context is cancelled (ctx cancel, `svc stop`/`svc restart`, service
	// manager shutdown). Zero means use the default (defaultShutdownGrace).
	//
	// The stop request reaches the worker as a SIGTERM on Unix, and on Windows as a
	// named Windows event the worker waits on (NOT a CTRL_BREAK console event, which
	// cannot be delivered to a worker spawned with CREATE_NO_WINDOW — see
	// prepareGracefulShutdown / watchGracefulShutdown in worker_signal_windows.go).
	// The event carries a restrictive DACL (creator + SYSTEM + Administrators), so
	// an unprivileged local process cannot use it to trigger a shutdown.
	// daemon.RunWorker already listens for both mechanisms.
	ShutdownGrace time.Duration

	// MonitorCmd / WorkerCmd are the hidden Cobra command names. Default __monitor/__worker.
	MonitorCmd string
	WorkerCmd  string

	// GracefulStop, when non-nil, is called by Stop (and therefore `service stop`) to
	// ask the RUNNING daemon to shut down cleanly before any forced kill. The
	// consumer implements however it talks to its own daemon (IPC, socket, HTTP,
	// signal, ...). It should return promptly once the request is acknowledged; it
	// need not wait for the process to actually exit — Stop polls that itself (via
	// Status) up to StopTimeout. When nil, Stop force-kills immediately (today's
	// behavior), so existing consumers are unaffected. Optional.
	GracefulStop func(ctx context.Context) error

	// StopTimeout bounds how long Stop (and `service stop`) waits for the process to
	// actually exit after GracefulStop before falling back to a forced kill. Ignored
	// when GracefulStop is nil. Zero means use the default (defaultStopTimeout).
	StopTimeout time.Duration

	// Serve is the worker body — the actual long-running process. Required.
	Serve func(ctx context.Context, p Ports) error

	// Logger receives structured lifecycle events (startup, restart, crash,
	// shutdown, ...). When nil, slog.Default() is used.
	Logger *slog.Logger

	// OnRestart, when set, is called by the monitor each time it restarts the worker
	// after a crash (or a failed upgrade re-exec), with the triggering exit code and the
	// consecutive-restart attempt count. It runs in the monitor process on the monitor
	// goroutine — keep it cheap and non-blocking (e.g. bump a metric). Optional.
	OnRestart func(code ExitStatus, attempt int)
}

// validatePort rejects a port outside the TCP range. Zero is allowed: it is the
// "unset" sentinel that withDefaults fills with the compiled-in default.
func validatePort(name string, port int) error {
	if port < 0 || port > 65535 {
		return fmt.Errorf("daemon: %s %d out of range (want 0-65535)", name, port)
	}

	return nil
}

// validate checks consumer-supplied fields that reach the OS (ports used for bind,
// ServiceName used as a task/registry/service name). It is called by AttachCommands
// after withDefaults so bad input fails fast at wiring time rather than at spawn.
func (o Options) validate() error {
	if err := validatePort("HTTPPort", o.HTTPPort); err != nil {
		return err
	}

	if err := validatePort("GRPCPort", o.GRPCPort); err != nil {
		return err
	}

	if o.ServiceName != "" {
		if err := validateServiceName(o.ServiceName); err != nil {
			return err
		}
	}

	return nil
}

// logger returns the configured logger, or slog.Default() when none is set.
func (o Options) logger() *slog.Logger {
	if o.Logger != nil {
		return o.Logger
	}

	return slog.Default()
}

// withDefaults returns a copy with zero-valued fields filled in.
func (o Options) withDefaults() Options {
	// Derive portsExplicit from the consumer's choice BEFORE filling defaults: a
	// non-default port means they overrode it, so the monitor must forward it to
	// the worker (buildMonitorArgs). Without this the flag is unreachable for any
	// external caller and the worker silently reverts to the compiled-in defaults.
	//
	// The rule is "non-default", not "non-zero", so the derivation is idempotent:
	// withDefaults runs twice in the real flow (AttachCommands then Start), and a
	// second pass over already-defaulted 9500/9501 ports must not flip the flag.
	// Setting a port to exactly its default is treated as implicit — forwarding it
	// would be identical to the worker's own default, so nothing is lost. We only
	// ever OR the flag true, never reset it (preserves a caller-set value).
	if (o.HTTPPort != 0 && o.HTTPPort != DefaultHTTPPort) ||
		(o.GRPCPort != 0 && o.GRPCPort != DefaultGRPCPort) {
		o.portsExplicit = true
	}

	if o.HTTPPort == 0 {
		o.HTTPPort = DefaultHTTPPort
	}

	if o.GRPCPort == 0 {
		o.GRPCPort = DefaultGRPCPort
	}

	o = o.withTimingDefaults()

	if o.MonitorCmd == "" {
		o.MonitorCmd = "__monitor"
	}

	if o.WorkerCmd == "" {
		o.WorkerCmd = "__worker"
	}

	if o.ServiceName == "" {
		o.ServiceName = o.BinaryName
	}

	if o.DataDir == "" {
		cache, err := os.UserCacheDir()
		if err != nil || cache == "" {
			cache = os.TempDir()
		}

		o.DataDir = filepath.Join(cache, o.BinaryName)
	}

	return o
}

// withTimingDefaults fills the guard/grace/timeout duration fields. Split out of
// withDefaults purely to keep that function's cyclomatic complexity down — it
// carries no independent ordering requirement relative to the rest of
// withDefaults.
func (o Options) withTimingDefaults() Options {
	if o.GuardSize == 0 {
		o.GuardSize = defaultGuardSize
	}

	if o.GuardWindow == 0 {
		o.GuardWindow = defaultGuardWindow
	}

	if o.ShutdownGrace == 0 {
		o.ShutdownGrace = defaultShutdownGrace
	}

	if o.StopTimeout == 0 {
		o.StopTimeout = defaultStopTimeout
	}

	return o
}
