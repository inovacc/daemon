# Feature Requests

## Implemented

- **Monitor→worker supervisor** — a monitor process spawns and supervises an inner
  worker, restarting it on crash under a sliding-window fork-loop guard + exponential
  backoff. `ExitRestart` bypasses the guard; a failed upgrade re-exec backs off like a crash.
- **Detached background lifecycle** — `Start`/`Stop` daemonize a detached `__monitor`,
  with a post-spawn health wait that surfaces an unconfirmed start (`ErrHealthCheckTimeout`).
  `stop` is idempotent when nothing is running.
- **OS-service registration** — the `svc` group (install/uninstall/start/stop/restart/status)
  via kardianos/service; every elevated verb gated by `RequirePrivilege` → exit `5`.
- **Windows launch-at-logon** — the `autostart` group (enable/disable/status) via the
  registry Run key or Task Scheduler, plus the combined `svc install --autostart` trigger.
- **Exit-code protocol** — `ExitCodeFor` maps `ErrNeedsPrivilege` (even wrapped) → exit 5;
  restart/upgrade codes drive the supervisor.
- **Input validation** — ports and `ServiceName` validated at wiring time (`AttachCommands`).
- **Structured logging** — slog throughout, with an injectable `Options.Logger`.
- **Restart observability hook** — optional `Options.OnRestart(code, attempt)` callback
  fired on each crash-restart, so consumers can export metrics.

## Proposed

- **Opt-in gRPC daemon path** — server + idle auto-shutdown (reintroduces `IdleTimeout`,
  wired) + discovery/auto-start, lifted from kody. (ROADMAP Phase 2 / BACKLOG P2.)
