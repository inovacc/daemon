# Backlog
<!-- rev:002 -->

## Priority Levels
| Priority | Timeline |
|----------|----------|
| P1 | This sprint |
| P2 | This quarter |
| P3 | Future |

## Items

- **Priority:** P1 — **Category:** CI / Release — **Effort:** Small
  - CI `Test` job is RED on `main`: workflows pin `go-version: '1.21'` (`test.yml`, `build.yml`)
    while `go.mod` declares `go 1.25`. Bump the pin to `1.25` to make CI green. Root cause of the
    two D-grade maturity dimensions (CI/CD + Stability). Verifying green requires a push.
    See `docs/analysis/MATURITY.md`.

- **Priority:** P2 — **Category:** Feature — **Effort:** Large
  - Optional gRPC daemon path (server + IdleTracker + discovery) lifted from kody, behind an
    opt-in Option. Enables the thin-client-daemon use case. (ROADMAP Phase 2.)

- **Priority:** P2 — **Category:** Tech Debt — **Effort:** Medium
  - DEPRECATION: now the `daemon` module lands, migrate weaver + kody to consume it and mark their
    in-tree supervisor/serverinfo copies `Deprecated:` with a ≥30-day removal date.

- **Priority:** P2 — **Category:** CI / Test — **Effort:** Medium
  - CI has no lint/vet gate and runs tests on ubuntu only; the Windows/darwin platform code
    (autostart, schtasks, registry, detach, stop) is cross-compiled but never executed. Add a
    `go vet` + golangci-lint job and a `windows-latest`/`macos-latest` test matrix.

## Resolved

- **P1 · CI/Lint** — golangci-lint red (7 issues: `any` shadow + 6× wsl_v5). ✅ 2026-07-11, `7b04772` (H-08/H-09) — now 0 issues.
- **P2 · Observability** — `Stop()` discarded `serverinfo.Remove()` error. ✅ 2026-07-11, `406ed72` (H-20) — logged as a non-fatal warning.
- **P2 · Test Coverage** — command handlers, `RunMonitor`, `svc` status branches. ✅ 2026-07-11, `6b6dad1`/`3b73426`/`2711d6f` (COV-02/03/04) — root pkg 72.8%→78.0%, total 76.7%.
- **P3 · Security** — validate consumer-supplied `ServiceName` charset. ✅ 2026-07-11, `fd5e63f` (H-06 / SEC-01).
- **P3 · Tech Debt** — fate of the `cmd/daemon` reference binary. ✅ dropped for a pure-library module (ADR-0002); wiring reference in `example_test.go`.
