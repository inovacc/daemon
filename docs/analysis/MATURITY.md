# Maturity Rating — github.com/inovacc/daemon

**Overall stage: 4 / 5 (Hardened)** · **Target: 4** · Evidence-based, re-rated 2026-07-11
after the full `/harden` loop (all 22 checklist items; H-12 skipped-optional).

Small pure-library that lets a consumer run its app as an OS service. Freshly flattened
(ADR-0002). Verified: `go vet` clean; `golangci-lint run` **0 issues**; `go build` clean on
windows/linux/darwin; `go test -race` PASS; total coverage **76.7%** (root pkg **78.0%**).

## Per-dimension

| # | Dimension | Stage | Evidence | Delivered |
|---|-----------|:-----:|----------|-----------|
| 1 | Correctness / Testing | 4 | root pkg 78.0% coverage; race+vet clean; autostart_windows logic funcs, RunMonitor, command handlers, svcStatus all covered | COV-01/02/03/04 |
| 2 | Error handling | 4 | %w/sentinels; ErrHealthCheckTimeout surfaces unconfirmed start; taskkill/kill diagnostics folded in; stop idempotent; remove-error logged | ERR-01/02/03/04/05/06 |
| 3 | Concurrency / lifecycle | 4 | race clean; single-goroutine supervisor; ctx-tied child kill; bounded-wait/single-Start/guard invariants documented | CON-01/02/03 |
| 4 | Architecture | 4 | Clean one-way imports, hidden internal/, narrow seams; ports contract reachable + tested; reexec seam relocated | ARCH-01/02 |
| 5 | Security | 4 | Arg-slice exec, integer pid, every elevated write gated by RequirePrivilege; ServiceName charset validated | SEC-01 |
| 6 | Build / CI / tooling | 4 | 3-OS build, vet clean, Taskfile, CI; `golangci-lint run ./...` exits 0 | LINT-01/02 |
| 7 | Dependencies | 4 | 3 direct (cobra, kardianos/service, x/sys), tidy, none stdlib-replaceable | DEP-01 (recorded) |
| 8 | Documentation / DX | 4 | All exports godoc'd, two runnable examples (incl. autostart), README, ADRs, AUTOSTART.md, 0 TODO | DX-01 |
| 9 | Observability | 4 | Structured slog throughout; Stop() now logs the pid-file removal failure | ERR-06 |
| 10 | Release / distribution | 3 | Library, git-tag releases; pre-1.0, no tagged release yet | Out of hardening scope (needs a tagged release + push) |

## Status

**All 9 in-scope dimensions at stage 4; hardening checklist 100% complete** (22/22, H-12
skipped-optional). The only dimension below target is **#10 Release/distribution**, which is
explicitly out of hardening scope — reaching stage 4 there requires cutting a tagged release
and pushing, neither of which the hardening loop performs (push is unauthorized this session).

Not formally **HARDENED** by the strict exit bar (every dimension ≥ target) solely because of
the out-of-scope release dimension. All code-quality dimensions the loop owns are at target.
