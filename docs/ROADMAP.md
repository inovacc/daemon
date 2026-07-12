# Roadmap
<!-- rev:002 -->

## Current Status
**Overall Progress:** ~75% — Supervisor, detached lifecycle, OS-service registration, and
Windows autostart all implemented and unit-tested; full hardening loop landed (maturity stage 4).
Remaining: optional gRPC daemon path, the weaver/kody migration, and the first tagged release
(gated on a green CI `main`). Pure library (ADR-0002).

## Test Coverage (`go tool cover`)
| Package | Coverage |
|---------|----------|
| `github.com/inovacc/daemon` (root) | 78.0% |
| `internal/serverinfo` | 60.0% |
| **Total** | **76.7%** (target 80%) |

Run `task test:cover`. Untested paths are tracked in [BACKLOG.md](BACKLOG.md) (COV-02/03/04).

## Phases

### Phase 1: Foundation [DONE]
- [x] Project scaffold (structure, tooling, CI, BSD-3 license)
- [x] Service-layer design spec (`docs/SERVICE_LIFECYCLE.md`) + current-state `docs/ARCHITECTURE.md`
- [x] Public library API at the module root (Options, AttachCommands, RunMonitor/RunWorker)
- [x] `internal/serverinfo` (monitor-PID file: write/read/IsRunning + stale self-heal)

### Phase 2: Core Features [DONE]
- [x] Monitor restart loop + sliding-window fork-loop guard + exponential backoff (unit-tested)
- [x] `__monitor` / `__worker` hidden Cobra commands + arg-builders (monitor never carries worker role)
- [x] Detached `service start` (daemonize → spawn `__monitor`) + env-guard self-spawn protection
- [x] Platform detach + stop (taskkill `/T /F` | SIGTERM group) build-tagged files; window hiding
- [x] kardianos/service install/uninstall integration (`svc` group)
- [x] Windows launch-at-logon (registry Run key / Task Scheduler) + `svc install --autostart` combined trigger
- [ ] gRPC daemon path (server + IdleTracker + discovery) lifted from kody
- [ ] Integration tests: real worker spawn, crash→restart, TestMain hard timeout

### Phase 3: Polish & Release [IN PROGRESS]
- [x] Hardening pass — Phase 1 (Stabilize): coverage seams (H-01), ports contract (H-02),
      unconfirmed-start sentinel (H-03), taskkill diagnostics (H-04). See `docs/analysis/`.
- [x] Hardening Phases 2–3 (lint green, observability, remaining coverage) → stage-4 maturity.
      All 22 checklist items done; maturity re-rated to stage 4 (`docs/analysis/MATURITY.md`).
- [ ] Fix CI go-version pin (1.21→1.25) → green `main`, then cut the first `v0.x` tag
- [ ] Port weaver and kody onto the module (behind deprecation dates)
- [ ] Stress/zombie tests + TestMain hard timeouts
- [ ] 80%+ coverage, CI green, v1.0.0 release
