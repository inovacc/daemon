# daemon
<!-- rev:004 -->

[![Go Reference](https://pkg.go.dev/badge/github.com/inovacc/daemon.svg)](https://pkg.go.dev/github.com/inovacc/daemon)
[![Go Version](https://img.shields.io/github/go-mod/go-version/inovacc/daemon)](go.mod)
[![License: BSD-3-Clause](https://img.shields.io/badge/License-BSD_3--Clause-blue.svg)](LICENSE)

A reusable Go **library** that gives any application the ability to run as a
long-lived background service — foreground supervisor (monitor → worker with a
fork-loop guard), OS-service registration, privilege handling, and Windows
launch-at-logon (Startup / Task Scheduler).

It is library-first: there is no binary in this module. You wire it into your own
Cobra root command and ship your own executable.

## Install

```bash
go get github.com/inovacc/daemon@latest
```

## Usage

```go
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/inovacc/daemon"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{Use: "myapp"}

	// serve is your long-running worker body; it must block until ctx is cancelled.
	serve := func(ctx context.Context, p daemon.Ports) error {
		slog.Info("serving", "http", p.HTTP, "grpc", p.GRPC)
		<-ctx.Done()
		return nil
	}

	if err := daemon.AttachCommands(root, daemon.Options{
		BinaryName: "myapp",
		Serve:      serve,
	}); err != nil {
		cobra.CheckErr(err)
	}

	if err := root.Execute(); err != nil {
		os.Exit(daemon.ExitCodeFor(err)) // maps privilege failures to exit 5
	}
}
```

`AttachCommands` wires these command groups onto your root:

| Command | What it does |
|---------|--------------|
| `service` / `service start` / `stop` / `status` | Foreground supervisor (monitor + worker) and detached background lifecycle |
| `svc install` / `uninstall` / `start` / `stop` / `restart` / `status` | Register with the OS init system (systemd / launchd / Windows service) — privileged |
| `svc install --autostart` | One step: install the elevated service **and** an elevated logon trigger that starts it |
| `autostart enable` / `disable` / `status` | Windows launch-at-logon via registry Run key or Task Scheduler (`--elevated` for all-users/SYSTEM) |

Privileged verbs are gated by an elevation check: without admin/root they print
re-run guidance and exit `5` (`ExitNeedsPrivilege`) without touching the system.

Windows autostart details (Startup vs Task Scheduler, the Google-style elevated
service + trigger shape) are in [docs/AUTOSTART.md](docs/AUTOSTART.md).

### Exit codes

`service start` / `stop` / `status` return a distinct exit code for each daemon
state, not just 0/1, so scripts can branch on them without parsing stdout:

| Code | Constant | When |
|------|----------|------|
| `0` | `ExitSuccess` | Clean success |
| `1` | `ExitError` | Generic failure |
| `5` | `ExitNeedsPrivilege` | A privileged `svc`/`autostart --elevated` verb ran unprivileged |
| `6` | `ExitAlreadyRunning` | `service start` against an already-running daemon |
| `7` | `ExitNotRunning` | `service stop` / `service status` against an idle daemon |

**Breaking change in 0.2.0:** prior to 0.2.0, `service status`/`stop` against an
idle host and `service start` against a live one printed a friendly message and
exited `0`. They now exit `6`/`7` (still printing the same message) so this state
is scriptable. If you depend on the old exit-`0`-always behavior, check
`errors.Is(err, daemon.ErrNotRunning)` / `errors.Is(err, daemon.ErrAlreadyRunning)`
in your own wrapper before mapping to your exit code.

### Checking daemon status

```go
running, pid, err := daemon.Status(daemon.Options{BinaryName: "myapp"})
```

`Status` verifies the recorded monitor PID is actually alive (not just that a
server.json file exists), so a crashed monitor's stale record reads as `running=false`.

### Graceful stop

By default `Stop`/`service stop` force-kills the daemon immediately. To give your
worker a chance to flush/checkpoint/release locks first, set:

```go
daemon.Options{
	// ... BinaryName, Serve, etc.

	// ShutdownGrace bounds how long the MONITOR waits for the WORKER to exit on
	// its own after a context-cancel-triggered stop (ctx cancel, svc stop/restart,
	// service-manager shutdown) before force-killing it. The worker is signalled
	// with a named event on Windows / SIGTERM on Unix; daemon.RunWorker already
	// listens for both (a console-control event was deliberately NOT used on
	// Windows — it requires the sender and receiver to share a console, which does
	// not hold for every deployment topology; verified empirically).
	ShutdownGrace: 20 * time.Second,

	// GracefulStop, when set, is called by Stop/`service stop` to ask the RUNNING
	// daemon (the whole monitor+worker tree) to shut down cleanly BEFORE any forced
	// kill — e.g. over your own IPC/socket/HTTP admin endpoint. Stop then polls the
	// monitor's actual PID liveness (not just "the hook returned") up to
	// StopTimeout, and only force-kills if it is still alive at that point.
	GracefulStop: func(ctx context.Context) error {
		return myIPCClient.RequestShutdown(ctx)
	},
	StopTimeout: 30 * time.Second,
}
```

### Restart guard: circuit breaker + backoff

The monitor protects against fork/spawn loop hell with two composable
primitives, both exported so you can also use them standalone in your own
supervision code:

- **`Breaker`** — a sliding-window circuit breaker. Once `MaxRestarts` crash
  events land inside the trailing `Window`, it trips to `BreakerOpenTerminal`
  and stays tripped for the process's lifetime (no half-open / auto-reset —
  a process crash-looping that fast should not silently keep retrying).
- **`Backoff`** — jittered exponential backoff: `delay = min(Base *
  Multiplier^attempt, Cap)`, optionally randomized by `±Jitter` (a fraction
  in `[0, 1)`).

`Options.GuardSize` / `Options.GuardWindow` (the pre-existing fields) still
work exactly as before and map onto `BreakerConfig{MaxRestarts: GuardSize,
Window: GuardWindow}`. For richer control, set the new additive fields:

```go
breakerCfg := daemon.BreakerConfig{MaxRestarts: 8, Window: 2 * time.Minute}
backoffCfg := daemon.BackoffConfig{
	Base:       500 * time.Millisecond,
	Cap:        30 * time.Second,
	Multiplier: 2.0,
	Jitter:     0.25, // ±25%, avoids restart-storm thundering herds
}

daemon.Options{
	// ... BinaryName, Serve, etc.
	Breaker: &breakerCfg,
	Backoff: &backoffCfg,
}
```

Either pointer may be set independently; any zero-valued field on a set
`BreakerConfig`/`BackoffConfig` falls back to the corresponding
`GuardSize`/`GuardWindow` value (for `Breaker`) or to the legacy default
curve (for `Backoff`). When both `Breaker` and `Backoff` are left `nil`
(the default), the restart guard behaves **exactly** as it did before this
primitive existed: a deterministic, zero-jitter `1s, 2s, 4s, ... capped at
60s` backoff curve and a `GuardSize`-events-in-`GuardWindow` trip point — no
behavior change for existing consumers.

`Breaker` and `Backoff` originated in [`slonik`](https://github.com/inovacc/slonik)
(a sibling project's managed-Postgres supervisor), which built them with zero
Postgres coupling; they were ported here as generic, first-class daemon
primitives.

### Routing supervisor-command exit codes (IMPORTANT)

The hidden `__monitor`/`__worker` commands communicate purely through process exit
codes (the `ExitStatus` protocol). If your `main()` has its OWN exit-code
convention for your app's commands, you **must** route the monitor/worker command's
error through `daemon.ExitCodeFor` and everything else through your own mapper —
mixing them up misreads a clean shutdown as a crash and can trip an unbounded
restart loop:

```go
cmd, err := root.ExecuteC()
executed, _ := root.Find(os.Args[1:])

if daemon.IsSupervisorCommand(executed, opts) {
	os.Exit(daemon.ExitCodeFor(err)) // stays in the monitor<->worker protocol
}

os.Exit(myApp.ExitCodeFor(err)) // your own contract for every other command
```

If your app has no exit-code convention of its own, the simple `os.Exit(daemon.ExitCodeFor(err))`
from the Usage example above is sufficient — `IsSupervisorCommand` is only needed
when two different exit-code contracts coexist in the same binary.

## Development

```bash
task test    # race + coverage
task check   # fmt, vet, lint, build, test
```

## License

BSD 3-Clause. See [LICENSE](LICENSE).
