# Backlog
<!-- rev:003 -->

## Priority Levels
| Priority | Timeline |
|----------|----------|
| P1 | This sprint |
| P2 | This quarter |
| P3 | Future |

## Items

- **Priority:** P2 — **Category:** Release — **Effort:** Small — **[BLOCKED: needs a push]**
  - Cut the first `v0.x.0` tag + add a `release.yml`; pin CI actions to SHAs. Gated on a green
    CI `main` (the go-version fix must land upstream first). Converts the two D-grade maturity
    dimensions. See `docs/analysis/MATURITY.md`.

- **Priority:** P2 — **Category:** Feature — **Effort:** Large — **[BLOCKED: needs kody source + spec]**
  - Optional gRPC daemon path (server + IdleTracker + discovery) lifted from kody, behind an
    opt-in Option. Enables the thin-client-daemon use case. (ROADMAP Phase 2.) Reintroduces the
    removed `IdleTimeout` field, wired to the idle tracker.

- **Priority:** P2 — **Category:** Tech Debt — **Effort:** Medium — **[BLOCKED: needs external repos]**
  - DEPRECATION: migrate weaver + kody to consume this module and mark their in-tree
    supervisor/serverinfo copies `Deprecated:` with a ≥30-day removal date.

- **Priority:** P2 — **Category:** Observability — **Effort:** Medium
  - Expose restart/crash counters (a Stats hook) — observability today is log-only, no metrics.

- **Priority:** P2 — **Category:** Test — **Effort:** Medium
  - Integration test: real worker spawn → crash → restart, with a TestMain hard timeout so a
    hung supervisor can't wedge CI. (ROADMAP Phase 2/3.)

- **Priority:** P3 — **Category:** Code-Quality — **Effort:** Medium
  - Re-enable gosec + a complexity linter (gocyclo/funlen at a lenient threshold) — the green
    golangci-lint run does not currently certify security statics or bounded complexity.

## Resolved

- **P1 · CI/Release** — Test job red on `main` (go-version 1.21 vs go.mod 1.25). ✅ 2026-07-12, `e6a71cf` — switched both workflows to `go-version-file: go.mod` (drift-proof). *Green verification needs a push.*
- **P2 · CI/Test** — no lint/vet gate; tests ubuntu-only. ✅ 2026-07-12, `e6a71cf` — added win/macOS test matrix + go vet + golangci-lint job.
- **P1 · CI/Lint** — golangci-lint red (7 issues). ✅ 2026-07-11, `7b04772` (H-08/H-09).
- **P2 · Observability** — `Stop()` discarded `serverinfo.Remove()` error. ✅ 2026-07-11, `406ed72` (H-20).
- **P2 · Test Coverage** — command handlers, `RunMonitor`, `svc` status branches. ✅ 2026-07-11, `6b6dad1`/`3b73426`/`2711d6f` (COV-02/03/04).
- **P3 · Security** — validate consumer-supplied `ServiceName` charset. ✅ 2026-07-11, `fd5e63f` (H-06 / SEC-01) + `faff04f` (ports + kardianos path).
- **P3 · Tech Debt** — fate of the `cmd/daemon` reference binary. ✅ dropped for a pure-library module (ADR-0002).
