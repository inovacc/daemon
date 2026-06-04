package daemon

import (
	"context"
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

	// IdleTimeout, when > 0, shuts the worker down after inactivity (gRPC path).
	IdleTimeout time.Duration

	// GuardSize / GuardWindow tune the fork-loop guard (abort after N restarts in window).
	GuardSize   int
	GuardWindow time.Duration

	// MonitorCmd / WorkerCmd are the hidden Cobra command names. Default __monitor/__worker.
	MonitorCmd string
	WorkerCmd  string

	// Serve is the worker body — the actual long-running process. Required.
	Serve func(ctx context.Context, p Ports) error

	// Logger receives structured lifecycle events (startup, restart, crash,
	// shutdown, ...). When nil, slog.Default() is used.
	Logger *slog.Logger
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
	if o.HTTPPort == 0 {
		o.HTTPPort = DefaultHTTPPort
	}
	if o.GRPCPort == 0 {
		o.GRPCPort = DefaultGRPCPort
	}
	if o.GuardSize == 0 {
		o.GuardSize = defaultGuardSize
	}
	if o.GuardWindow == 0 {
		o.GuardWindow = defaultGuardWindow
	}
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
