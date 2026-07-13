# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

_Nothing yet._

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
