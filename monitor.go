package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/inovacc/daemon/internal/serverinfo"
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
			log.Info("worker requested binary upgrade", slog.Int("code", code.AsInt()))
			log.Info("re-executing for binary upgrade")
			// Reuse the process's ORIGINAL invocation args so the re-execed image keeps
			// its role: a __monitor process re-execs as __monitor, a future "svc run"
			// OS-service re-execs as "svc run". Do NOT use buildMonitorArgs() here.
			if err := reexecFn(os.Args[1:]); err != nil {
				// On Unix syscall.Exec replaces this image and never returns; on
				// Windows reexecSelf exits the process. Reaching here means re-exec
				// FAILED — degrade to a restart so we never kill the service, but treat
				// it as a CRASH (guard + backoff), not an intentional ExitRestart. A
				// worker that keeps requesting an un-performable upgrade must back off
				// and ultimately trip the fork-loop guard, not busy-loop with no delay.
				log.Error("re-exec failed; falling back to restart", slog.Any("err", err))

				if stop, err := m.handleCrash(ctx, log, code, &attempt); stop {
					return err
				}

				continue
			}
			// Unreachable: a successful re-exec replaced/exited the process above.
			continue
		// ExitError, ExitNeedsPrivilege, and any unknown code are all treated as a crash:
		// the worker is restarted subject to the fork-loop guard and backoff.
		case ExitError, ExitNeedsPrivilege:
			if stop, err := m.handleCrash(ctx, log, code, &attempt); stop {
				return err
			}
		default:
			if stop, err := m.handleCrash(ctx, log, code, &attempt); stop {
				return err
			}
		}
	}
}

// handleCrash applies the crash-restart policy for a non-clean, non-restart, non-upgrade
// exit code. It returns stop=true (with the loop's return value) when the monitor must
// exit — either because the context was cancelled (err=nil) or the fork-loop guard tripped
// (err set). When stop=false the worker should be respawned after the backoff sleep, and
// *attempt is incremented.
func (m *monitor) handleCrash(ctx context.Context, log *slog.Logger, code ExitStatus, attempt *int) (bool, error) {
	select {
	case <-ctx.Done():
		log.Info("monitor stopping", slog.String("reason", "context canceled"))
		return true, nil
	default:
	}

	if m.guard.isLoop(time.Now()) {
		log.Error("restart loop detected; aborting",
			slog.Int("crashes", m.o.GuardSize), slog.Duration("window", m.o.GuardWindow))

		return true, fmt.Errorf("restart loop detected: worker crashed %d times within %s — aborting",
			m.o.GuardSize, m.o.GuardWindow)
	}

	d := m.guard.backoff(*attempt)
	log.Warn("worker crashed; restarting",
		slog.Int("code", code.AsInt()), slog.Int("attempt", *attempt+1), slog.Duration("backoff", d))
	m.sleep(d)

	*attempt++

	return false, nil
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
