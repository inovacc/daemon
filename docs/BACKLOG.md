# Backlog
<!-- rev:007 -->

## Priority Levels
| Priority | Timeline |
|----------|----------|
| P1 | This sprint |
| P2 | This quarter |
| P3 | Future |

## Items

_v0.1.0 is released and CI is green on `main` (3-OS matrix + lint + govulncheck gates).
Everything else is either a dependabot PR awaiting review or blocked on external repos._

- **Priority:** P2 — **Category:** Deps — **Effort:** Medium
  - Review + merge the 6 dependabot PRs opened 2026-07-12. Four are **major** action bumps
    (checkout v4→v7, setup-go v5→v6, codecov v5→v7, golangci-lint-action v8→v9) that need CI
    verification before merge; two are Go deps (`x/sys` 0.44→0.47, `kardianos/service`
    1.2.2→1.3.0). Merge the Go deps first, then the actions one at a time.

- **Priority:** P2 — **Category:** Feature — **Effort:** Large — **[BLOCKED: needs kody source + spec]**
  - Optional gRPC daemon path (server + IdleTracker + discovery) lifted from kody, behind an
    opt-in Option. Enables the thin-client-daemon use case. (ROADMAP Phase 2.) Reintroduces the
    removed `IdleTimeout` field, wired to the idle tracker.

- **Priority:** P2 — **Category:** Tech Debt — **Effort:** Medium — **[BLOCKED: needs external repos]**
  - DEPRECATION: migrate weaver + kody to consume this module and mark their in-tree
    supervisor/serverinfo copies `Deprecated:` with a ≥30-day removal date.

## Resolved

- **P2 · Security/CI** — govulncheck was local-only; repo lacked dependabot + templates. ✅ 2026-07-12, `401696f` — govulncheck is now a CI gate, dependabot covers gomod + github-actions (required, since actions are SHA-pinned), added issue/PR templates + CODEOWNERS. Also fixed the codecov step, which had been **failing silently on every run** ("Token required - not valid tokenless upload"): it now prints coverage to the CI summary and only uploads when a `CODECOV_TOKEN` secret exists.
- **P2 · Release** — release automation. ✅ 2026-07-12, `6b6e9a2` — tag-triggered `release.yml` + CI actions pinned to SHAs + codecov hardened. v0.1.0 GitHub Release published.
- **P2 · Observability** — restart/crash counters. ✅ 2026-07-12, `2ffb42d` — added the optional `Options.OnRestart` hook.
- **P2 · Test** — real spawn→crash→restart integration test + TestMain hard timeout. ✅ 2026-07-12, `49c2d90` — realSpawn 0%→81.8%, total →79.6%.
- **P3 · Code-Quality** — re-enable gosec + a complexity linter. ✅ 2026-07-12, `af65e16` — gosec + gocyclo on; dir 0750, pid overflow guard; by-design/test findings excluded.
- **P1 · CI/Release** — Test job red on `main` (go-version 1.21 vs go.mod 1.25). ✅ 2026-07-12, `e6a71cf` — switched both workflows to `go-version-file: go.mod` (drift-proof). *Green verification needs a push.*
- **P2 · CI/Test** — no lint/vet gate; tests ubuntu-only. ✅ 2026-07-12, `e6a71cf` — added win/macOS test matrix + go vet + golangci-lint job.
- **P1 · CI/Lint** — golangci-lint red (7 issues). ✅ 2026-07-11, `7b04772` (H-08/H-09).
- **P2 · Observability** — `Stop()` discarded `serverinfo.Remove()` error. ✅ 2026-07-11, `406ed72` (H-20).
- **P2 · Test Coverage** — command handlers, `RunMonitor`, `svc` status branches. ✅ 2026-07-11, `6b6dad1`/`3b73426`/`2711d6f` (COV-02/03/04).
- **P3 · Security** — validate consumer-supplied `ServiceName` charset. ✅ 2026-07-11, `fd5e63f` (H-06 / SEC-01) + `faff04f` (ports + kardianos path).
- **P3 · Tech Debt** — fate of the `cmd/daemon` reference binary. ✅ dropped for a pure-library module (ADR-0002).
