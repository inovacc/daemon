# Milestones
<!-- rev:002 -->

## v0.1.0 - Initial Setup [DONE]
- **Status:** Complete
- **Goals:**
  - [x] Project scaffolding (library shell)
  - [x] `daemon` public API (root package: Options, AttachCommands, RunMonitor/RunWorker)
  - [x] `internal/serverinfo` package

## v0.2.0 - Core Features [DONE]
- **Status:** Complete
- **Coverage:** 72.8% total
- **Goals:**
  - [x] Monitor loop + fork-loop guard
  - [x] `__monitor`/`__worker` command wiring + platform files (detach/stop/re-exec)
  - [x] Detached `service start` + kardianos/service `svc` group
  - [x] Windows launch-at-logon (Startup / Task Scheduler) + `svc install --autostart`

## v0.3.0 - Hardening [DONE]
- **Status:** Complete — all 22 checklist items landed; maturity re-rated to stage 4
- **Coverage:** 76.7% total (root 78.0%); target 80%
- **Goals:**
  - [x] Stabilize: autostart test seams, ports contract, unconfirmed-start sentinel, taskkill diagnostics
  - [x] Harden: golangci-lint green (0 issues), security/concurrency doc notes, error diagnostics
  - [x] Mature: command-handler + supervisor coverage, observability (Stop remove-error logged)

## v1.0.0 - First Stable Release [NOT STARTED]
- **Status:** Not Started
- **Goals:**
  - [ ] Optional gRPC daemon path (from kody)
  - [ ] weaver + kody migrated onto the module
  - [ ] 80%+ coverage, docs complete, CI green
