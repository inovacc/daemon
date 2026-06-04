package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/inovacc/daemon/pkg/serverinfo"
)

// monitor supervises the worker: it spawns the inner process, watches its exit code,
// and restarts it under the fork-loop guard. It owns server.json (with the MONITOR pid).
type monitor struct {
	o     Options
	guard *restartGuard
	info  *serverinfo.Store

	// spawn runs the worker child and returns its exit code. Injectable for tests.
	spawn func(ctx context.Context, args []string) int
	// sleep delays between crash restarts. Injectable for tests.
	sleep func(time.Duration)
}

func newMonitor(o Options) *monitor {
	return &monitor{
		o:     o,
		guard: newRestartGuard(o.GuardSize, o.GuardWindow),
		info:  serverinfo.NewStore(o.DataDir),
		spawn: realSpawn,
		sleep: time.Sleep,
	}
}

// RunMonitor runs the supervisor loop in the foreground until the worker exits cleanly,
// the context is cancelled, or the fork-loop guard aborts.
func RunMonitor(ctx context.Context, opts Options) error {
	return newMonitor(opts.withDefaults()).run(ctx)
}

func (m *monitor) run(ctx context.Context) error {
	log := m.o.logger().With(slog.String("role", "monitor"))
	addr := fmt.Sprintf("localhost:%d", m.o.GRPCPort)
	if err := m.info.Write(serverinfo.Info{
		Address: addr,
		Port:    m.o.GRPCPort,
		PID:     os.Getpid(),
		Version: m.o.Version,
	}); err != nil {
		return fmt.Errorf("write server info: %w", err)
	}
	log.Info("monitor started",
		slog.Int("pid", os.Getpid()), slog.String("version", m.o.Version), slog.String("address", addr))
	defer func() {
		_ = m.info.Remove()
		log.Info("monitor stopped")
	}()

	args := m.o.buildWorkerArgs()
	attempt := 0
	for {
		if ctx.Err() != nil {
			log.Info("monitor stopping", slog.String("reason", "context canceled"))
			return nil
		}
		log.Debug("starting worker", slog.Int("attempt", attempt))
		code := ExitStatus(m.spawn(ctx, args))
		switch code {
		case ExitSuccess:
			log.Info("worker exited cleanly", slog.Int("code", code.AsInt()))
			return nil
		case ExitRestart:
			log.Info("worker requested restart", slog.Int("code", code.AsInt()))
			attempt = 0
			continue
		case ExitUpgrade:
			// Re-exec lands in C3; for now this restarts the worker.
			log.Info("worker requested binary upgrade", slog.Int("code", code.AsInt()))
			attempt = 0
			continue
		default: // ExitError / any crash
			if ctx.Err() != nil {
				log.Info("monitor stopping", slog.String("reason", "context canceled"))
				return nil
			}
			if m.guard.isLoop(time.Now()) {
				log.Error("restart loop detected; aborting",
					slog.Int("crashes", m.o.GuardSize), slog.Duration("window", m.o.GuardWindow))
				return fmt.Errorf("restart loop detected: worker crashed %d times within %s — aborting",
					m.o.GuardSize, m.o.GuardWindow)
			}
			d := m.guard.backoff(attempt)
			log.Warn("worker crashed; restarting",
				slog.Int("code", code.AsInt()), slog.Int("attempt", attempt+1), slog.Duration("backoff", d))
			m.sleep(d)
			attempt++
		}
	}
}

// realSpawn executes the worker as a child of this process, inheriting stdio.
func realSpawn(ctx context.Context, args []string) int {
	exe, err := os.Executable()
	if err != nil {
		return ExitError.AsInt()
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		return ExitError.AsInt()
	}
	return ExitSuccess.AsInt()
}
