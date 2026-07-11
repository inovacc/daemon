package daemon

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/inovacc/daemon/internal/serverinfo"
)

// ErrAlreadyRunning is returned by Start when a live instance already exists.
var ErrAlreadyRunning = errors.New("daemon: already running")

// ErrNotRunning is returned by Stop when no live instance exists.
var ErrNotRunning = errors.New("daemon: not running")

// ErrHealthCheckTimeout is returned by Start (alongside the spawned pid) when the
// detached monitor was launched but did not write its serverinfo within
// healthWaitTimeout. The process exists, but readiness is UNCONFIRMED — a
// spawned-but-crashed monitor lands here, so callers must not treat it as success.
var ErrHealthCheckTimeout = errors.New("daemon: health check timed out; monitor did not report ready")

// spawnDetachedFn / stopProcessFn are seams overridden in tests; production points at
// the platform implementations (spawn_*.go / stop_*.go).
var (
	spawnDetachedFn = spawnDetached
	stopProcessFn   = stopProcess
)

// healthWaitTimeout bounds the post-spawn wait for the monitor to report ready
// (write serverinfo). A package var so tests can shrink it; production is 5s.
var healthWaitTimeout = 5 * time.Second

// childEnvName is the recursion-guard env var, e.g. "MY_APP" -> "MY_APP_DAEMON_CHILD".
func childEnvName(binaryName string) string {
	up := strings.ToUpper(binaryName)
	up = strings.Map(func(r rune) rune {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}

		return '_'
	}, up)

	return up + "_DAEMON_CHILD"
}

// Start daemonizes: it spawns a detached __monitor process and returns its pid.
// It returns ErrAlreadyRunning (with the live pid) when an instance is already up,
// and refuses to daemonize from within a daemon child (guarded by the env var).
func Start(o Options) (int, error) {
	o = o.withDefaults()

	guard := childEnvName(o.BinaryName)
	if os.Getenv(guard) != "" {
		return 0, errors.New("daemon: refusing to daemonize from within a daemon child")
	}

	store := serverinfo.NewStore(o.DataDir)
	if info := store.IsRunning(); info != nil {
		return info.PID, ErrAlreadyRunning
	}

	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}

	env := append(os.Environ(), guard+"=1")

	pid, err := spawnDetachedFn(exe, o.buildMonitorArgs(), env)
	if err != nil {
		return 0, fmt.Errorf("daemon: spawn: %w", err)
	}
	// TOCTOU health wait: the monitor writes serverinfo on startup.
	deadline := time.Now().Add(healthWaitTimeout)
	for time.Now().Before(deadline) {
		if store.IsRunning() != nil {
			return pid, nil
		}

		time.Sleep(50 * time.Millisecond)
	}

	// Spawned, but serverinfo never appeared — readiness is unconfirmed (the monitor
	// may still be starting, or may have crashed). Return the pid AND the sentinel so
	// the caller distinguishes this from a confirmed start instead of assuming success.
	return pid, ErrHealthCheckTimeout
}

// Stop reads the serverinfo (monitor) pid and terminates the daemon process tree.
func Stop(o Options) error {
	o = o.withDefaults()
	store := serverinfo.NewStore(o.DataDir)

	info := store.IsRunning()
	if info == nil {
		return ErrNotRunning
	}

	if err := stopProcessFn(info.PID); err != nil {
		return fmt.Errorf("daemon: stop pid %d: %w", info.PID, err)
	}

	_ = store.Remove()

	return nil
}
