# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

_Nothing yet._

## [0.3.0] - 2026-07-13

### Added
- **`Breaker` / `BreakerConfig`** (`breaker.go`) — a general-purpose,
  first-class exported sliding-window circuit breaker: once `MaxRestarts`
  events land inside the trailing `Window` it trips to `BreakerOpenTerminal`
  and stays tripped for the process's lifetime. Thread-safe.
- **`Backoff` / `BackoffConfig`** (`backoff.go`) — a general-purpose,
  first-class exported jittered exponential backoff: `delay = min(Base *
  Multiplier^attempt, Cap)`, randomized by `±Jitter`.
- **`Options.Breaker *BreakerConfig`** / **`Options.Backoff
  *BackoffConfig`** — additive Options fields letting a consumer opt into
  the richer Breaker/Backoff configuration (e.g. non-zero jitter, a
  different multiplier) instead of the legacy `GuardSize`/`GuardWindow`
  pair. Both types originated in [`slonik`](https://github.com/inovacc/slonik)
  (a sibling project's managed-Postgres supervisor), which built them with
  zero Postgres coupling; they are ported here, adapted, and re-tested as
  daemon-native primitives.

### Changed
- The monitor's restart loop (`monitor.go`'s `handleCrash`, via
  `restartGuard`) is now implemented **on top of** `Breaker` + `Backoff`
  instead of the old ad-hoc `restartGuard.isLoop`/`backoff` math. The
  `restartGuard` type itself, and its package-private `isLoop`/`backoff`
  methods, are unchanged in signature and are kept as a thin adapter — this
  refactor is implementation-internal.

### Back-compat (IMPORTANT for existing consumers, e.g. `indexer`)
- `Options.GuardSize` / `Options.GuardWindow` are **unchanged** — same
  names, same types, same defaults (`4` restarts / `60s` window), same
  trip semantics. When `Options.Breaker` is left `nil` (the default), the
  monitor derives `BreakerConfig{MaxRestarts: GuardSize, Window:
  GuardWindow}` from them exactly as the old `restartGuard` did.
- The default restart-delay curve is **unchanged**: when `Options.Backoff`
  is left `nil` (the default), the monitor uses a deterministic,
  zero-jitter `1s, 2s, 4s, 8s, ... capped at 60s` curve — bit-for-bit
  identical to the old `restartGuard.backoff` formula for every attempt
  value. No existing consumer observes a different restart delay or a
  different trip point unless it explicitly sets the new `Options.Breaker`
  / `Options.Backoff` fields.
- One micro-semantic note for anyone driving `Breaker`/`restartGuard`
  directly at exact window-boundary timestamps: the old ad-hoc guard kept
  an event only if it was *strictly after* `now-window` (`t.After(cutoff)`);
  `Breaker` prunes an event only if it is *strictly before* `now-window`
  (`t.Before(cutoff)`), i.e. an event landing exactly on the boundary is now
  kept instead of dropped. This is a one-nanosecond edge case that does not
  affect any existing test or any realistic crash-timing scenario (restart
  timestamps are never exactly `window` apart to sub-nanosecond precision in
  practice).

## [0.2.0] - 2026-07-13

Fixes six defects and adds three additive capabilities found integrating this
library into a real Windows-first consumer, moving generic supervisor concerns
(graceful-first stop, status query, exit-code routing) into the library so every
consumer gets them instead of hand-rolling them.

### Fixed
- **F1 (Windows, user-visible regression):** the worker no longer pops a phantom
  console window on every daemon start. `realSpawn` now gives the worker its own
  platform `SysProcAttr` (`workerSysProcAttr`): `CREATE_NO_WINDOW |
  CREATE_NEW_PROCESS_GROUP` on Windows (deliberately *not* `DETACHED_PROCESS` — the
  worker stays a true child so `exec.CommandContext` can `Wait()` on it and
  `taskkill /T` still reaps the tree), `Setpgid` on Unix.
- **F2:** `service <bogus>` (e.g. a typo'd `service restart`) no longer silently
  starts the daemon. The `service`, `__monitor`, and `__worker` commands now set
  `Args: cobra.NoArgs`, so an unrecognized trailing argument is a CLI error instead
  of being swallowed while `RunMonitor` still fires. `svc`/`autostart` were audited
  and found not to exhibit the defect (their leaf commands never invoke
  `RunMonitor`).
- **F5:** `Stop` (and the `taskkill`/`kill` platform layer) now confirms the
  process actually exited — polling PID liveness, bounded — instead of returning
  as soon as the kill/signal call itself succeeded. Fixes a race where `stop`
  could report success while the monitor was still alive, causing a following
  `start` to no-op into an empty system.
- **PID liveness on Unix (zombies):** `processAlive` no longer reports an exited-
  but-unreaped child (a *zombie*) as running. `kill(pid, 0)` succeeds for a zombie,
  so the old probe made F5's exit-confirmation poll hang until timeout whenever the
  caller was the process's parent — which `Start()` is, since `spawnDetached` uses
  `cmd.Start()` + `Release()` and never reaps. Linux now reads `/proc/<pid>/stat`
  and darwin queries `kern.proc.pid` via sysctl, both of which report the zombie
  state directly.
- **PID liveness on Unix (EPERM):** `processAlive` no longer reports a process that
  exists but which we lack permission to signal (`kill` → `EPERM`, e.g. a monitor
  running as root or another user) as *dead*. Only `ESRCH` now means gone.

### Added
- **F3:** `Status(o Options) (running bool, pid int, err error)` — exported query
  for whether the daemon is running and the monitor's PID, reusing the existing
  PID-liveness/self-healing logic in `internal/serverinfo`.
- **F4:** `Options.ShutdownGrace` — the monitor now gives the worker a grace period
  to shut down cleanly (a named Windows event on Windows — see the Security note
  below for why not `CTRL_BREAK` — SIGTERM on Unix) before force-killing on context
  cancellation (`svc stop`/`restart`, service-manager shutdown), instead of Go's
  previous default of an immediate `Process.Kill()`. A worker that exits cleanly
  after the signal still reports its real exit code to the monitor.
- **F6:** `Options.GracefulStop` + `Options.StopTimeout` — `Stop`/`service stop` now
  ask the running daemon to shut down cleanly via a consumer-supplied hook (IPC,
  socket, HTTP, ...) *before* any forced kill, then poll **actual PID liveness**
  (via `Status`, not just "the hook returned") up to `StopTimeout` before falling
  back to a forced kill. `GracefulStop` is nil by default, so existing consumers
  are unaffected (straight force-kill, unchanged).
- **F8:** `IsSupervisorCommand(cmd *cobra.Command, o Options) bool` — identifies the
  hidden `__monitor`/`__worker` commands, so a consumer with its own exit-code
  contract can route their errors through `daemon.ExitCodeFor` and everything else
  through its own mapper without hand-rolling the name comparison. See the README
  for the wiring pattern (getting it wrong causes a restart loop).
- New exit codes: `ExitAlreadyRunning` (6), `ExitNotRunning` (7) — see F7 below.

### Changed
- **F7 (BREAKING behavior):** `service start` against an already-running daemon,
  and `service stop`/`service status` against an idle one, now return the
  corresponding sentinel error (`ErrAlreadyRunning` / `ErrNotRunning`) — mapped by
  `ExitCodeFor` to exit `6`/`7` — instead of printing a friendly message and always
  exiting `0`. The printed text is unchanged; only the exit code / returned error
  changed, so scripts can now branch on daemon state. See README "Exit codes" for
  the full table and a migration note for callers that depended on the old
  exit-0-always behavior.

### Security
- **Graceful-shutdown event is DACL-restricted (Windows).** The F4 named event is
  created with an explicit *protected* DACL granting access only to the creating
  user, LocalSystem, and Administrators (SDDL
  `D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GA;;;<creator SID>)`), instead of the default
  security descriptor. A named kernel object with a default DACL can be opened by
  any process in the same session, so without this **any local process could open
  `Local\daemon-graceful-*` and signal it to trigger an unauthenticated graceful
  shutdown of the daemon** (local denial of service). Mirrors the named-pipe DACL
  hardening in the sibling `indexer` project.
- **No `CloseHandle` with an outstanding wait (Windows).** The worker-side waiter
  now blocks on the graceful event *and* a local "done" event via
  `WaitForMultipleObjects`, and cleanup signals `done` and joins the waiter before
  closing either handle. Previously a worker exiting for any reason other than the
  event firing would close a handle that still had a pending
  `WaitForSingleObject(INFINITE)` on it — a documented handle-reuse hazard.
- The Windows F4 mechanism deliberately avoids
  `GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, ...)` even though F1's
  `CREATE_NEW_PROCESS_GROUP` would make it possible in principle: CTRL events
  require the sender and receiver to share a console, which does not hold for a
  worker spawned with `CREATE_NO_WINDOW` (it gets its own, separate hidden
  console) — confirmed empirically (a live spawn+signal probe: the call reports
  success but the target never reacts) and is fragile across other deployment
  topologies (e.g. a monitor with redirected/absent stdio). A named Windows event
  has no such dependency and was verified to work reliably instead.

## [0.1.1] - 2026-07-13

**No library code changed** — v0.1.1 is functionally identical to v0.1.0. This release
only lifts the dependency floor and carries the repository's CI/security hardening.

### Changed
- Raise dependency minimums: `golang.org/x/sys` 0.44.0 → 0.47.0 and
  `github.com/kardianos/service` 1.2.2 → 1.3.0.
- CI: govulncheck is now a required gate; tests run on a windows/linux/macOS matrix with a
  lint gate; all GitHub Actions are pinned to commit SHAs and tracked by dependabot;
  releases are automated from tags.

## [0.1.0] - 2026-07-12

First tagged release. CI green on windows/linux/darwin; coverage 81.7%.

### Added
- Monitor→worker supervisor with a sliding-window fork-loop guard and exponential backoff.
- Detached background lifecycle: `Start` / `Stop`, with a post-spawn health wait that
  surfaces an unconfirmed start via `ErrHealthCheckTimeout`.
- OS-service registration (`svc` group: install/uninstall/start/stop/restart/status) via
  kardianos/service; every elevated verb gated by `RequirePrivilege` → exit `5`.
- Windows launch-at-logon (`autostart` group) via the registry Run key or Task Scheduler,
  plus the combined `svc install --autostart` trigger.
- Exit-code protocol: `ExitCodeFor` maps `ErrNeedsPrivilege` (even wrapped) to exit `5`.
- `Options.OnRestart` — optional callback fired on each crash-restart for metrics.
- Port and `ServiceName` validation at wiring time (`AttachCommands`).
- Runnable godoc examples for `Start`, `Stop`, and `RunMonitor`.
- Test depth: a real spawn→crash→restart integration test with a TestMain hard timeout,
  fuzz targets for the validators, benchmarks, and platform-leaf coverage (total ≥ 80%).

### Changed
- Flattened to a pure-library module at the package root; dropped `cmd/` and GoReleaser
  (see ADR-0002). Consumer wiring reference lives in `example_test.go`.
- `stop` is idempotent: stopping an already-stopped daemon exits `0` (was an error).
- CI reads the Go toolchain from `go.mod` and runs a Windows/macOS test matrix plus a
  `go vet` + golangci-lint gate.

### Fixed
- A failed upgrade re-exec now backs off through the crash guard instead of busy-looping.
- A corrupt/unreadable `server.json` is self-healed (removed) instead of wedging startup.
- `childEnvName` no longer maps distinct binaries (e.g. `my-app` / `my_app`) to one guard var.
- `taskkill` (Windows) and group/single-pid `kill` (unix) failures fold their diagnostics
  into the returned error.

### Security
- `server.json` is written `0600` (owner-only) instead of world-readable.
- Re-enabled gosec in CI; bumped `golang.org/x/sys` to v0.44.0 (clears GO-2026-5024) and
  cobra to v1.10.2.

### Removed
- The dead `Options.IdleTimeout` field (never wired; returns with the planned gRPC path).
