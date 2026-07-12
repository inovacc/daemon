# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

_Nothing yet._

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
