# Maturity Rating — github.com/inovacc/daemon

**Type:** pure Go 1.25 library (no binary) · **Stage: 4 / 5 (Hardened)** ·
**Weighted score: 84.9 / 100** · **Confidence: High** · 2026-07-12.

Independent 10-dimension audit (one auditor per dimension, real probes). Supersedes the
2026-07-11 hardening-loop rating. Every score cites a measured signal.

Baseline: 1698 prod / 1923 test LOC (test:prod > 1), 27 prod + 17 test files, 2 packages,
38 commits (20 in 30d), **0 git tags**, 0 TODO/nolint markers. Probes: `go test -race` PASS,
`go vet` clean, `golangci-lint run` 0 issues, coverage **76.7%** (root 78.0%, serverinfo 60.0%),
3-OS build clean, govulncheck 0 reachable vulns.

## Scorecard

| # | Dimension | Grade | Conf | Evidence (measured) |
|---|-----------|:-----:|:----:|---------------------|
| 1 | Architecture | **A** | High | One-way imports, no cycles; ARCH-01/02 fixed; minimal API; seams not leaked; 2 ADRs |
| 2 | Correctness | **A** | High | race PASS, vet clean; invariants pinned (guard off-by-one, sliding window, backoff cap, ctx-cancel) |
| 3 | Documentation | **A** | High | Full godoc on exports; 2 runnable Examples; ARCHITECTURE.md matches code; public-appropriate badges |
| 4 | Code-Quality | **A** | High | `.golangci.yml` default:all−curated (strict); 0 //nolint, 0 TODO, no `any`, no lib panic; 27 `%w` wraps |
| 5 | Testing | **B** | High | 76.7% total; all 0% funcs are syscall/exec leaves behind seams; serverinfo 60% caps below 80% |
| 6 | Security | **B** | High | Arg-slice exec (no injection); privilege gate on all 5 verbs; no secrets/unsafe; 1 *non-reachable* vuln |
| 7 | Operational | **B** | High | slog throughout, Logger injectable, guard+backoff+health-wait+self-heal; but **`IdleTimeout` dead**, no metrics |
| 8 | Dependencies | **B** | Med | 3 direct justified, shallow tree (10 mods, depth 3), 0 reachable vulns; cobra ~2 minors stale |
| 9 | CI/CD | **D** | Med | **Test job RED on `main`** (2 runs); root cause `go-version:'1.21'` vs `go.mod 1.25`; no lint gate; ubuntu-only |
| 10 | Stability / Release | **D** | High | **0 tags** (no release; consumers can't pin); CI red on main; high `fix()` churn + breaking flatten |

## Weighted arithmetic

Grade points A=100 B=82 C=65 D=48 F=25. Criticality weights (Σ = 35):

```
Correctness   A=100 ×5 = 500     CI/CD          D=48 ×3 = 144
Testing       B= 82 ×5 = 410     Code-Quality   A=100 ×3 = 300
Security      B= 82 ×4 = 328     Dependencies   B= 82 ×3 = 246
Architecture  A=100 ×4 = 400     Operational    B= 82 ×3 = 246
Documentation A=100 ×3 = 300     Stability      D= 48 ×2 =  96
                                 -------------------------------
Σ weighted = 2970    ÷ 35 = 84.86  →  84.9 / 100  →  Stage 4
```

Confidence **High**: 8/10 dimensions High (real probes); CI/CD + Dependencies Med only because
`gh run view` logs and `govulncheck -show verbose` couldn't run in this environment.

## Ranked weak points

1. **CI Test job red on `main`** (CI/CD D) — `test.yml:18`/`build.yml:22` pin Go 1.21 vs `go.mod` 1.25. Invalidates every downstream "green" (coverage, vuln, lint) claim.
2. **Zero releases** (Stability D) — 0 tags; consumers cannot pin a version.
3. **Re-exec busy-loop** (Correctness) — `monitor.go:101` degrades a failed re-exec to restart with `attempt=0; continue` and NO backoff → unbounded CPU spin. *Most dangerous defect, but isolated.*
4. **Dead `IdleTimeout`** (Operational) — declared `options.go:45`, never wired: advertised config that does nothing.
5. **1 non-reachable transitive vuln** (Security/Deps) — likely go-md2man/blackfriday via cobra's doc-gen; bump cobra to clear.
6. **serverinfo 60% coverage** (Testing) — Write/Read/Remove error branches uncovered, caps total below 80%.
7. **Uneven input validation** — ports unvalidated (neg/>65535); `validateServiceName` skips the `realOSService`→kardianos path.

## Route to stage 5 (leverage-ranked)

Leverage = unblock-fan-out × weight ÷ effort. Four S-effort Phase-1 fixes convert BOTH D dimensions.

**Phase 1 — Stabilize**
- **S1. Fix the Go-version pin** (1.21→1.25 in both workflows). Green Test → unblocks Testing (codecov) + Stability (green baseline → tag) + CI vuln gate. Effort S.
- **S2. Bump cobra v1.9+** + `govulncheck -show verbose`. Clears Security + Dependencies + CI vuln gate at once. Effort S.
- **S3. Validate port + service name in `withDefaults`** (reject neg/>65535; run validateServiceName on the kardianos path). Unblocks Security + Code-Quality. Effort S.
- **S4. Fix the re-exec busy-loop** (`monitor.go:101` — backoff, don't reset attempt). Safety, not leverage. Effort S.

**Phase 2 — Harden**
- H1. Add lint/vet gate + Windows/macOS test matrix (exercises the 0%-covered platform seams). Effort M.
- H2. Cover serverinfo Write/Read/Remove error branches → total ≥ 80%. Effort M.
- H3. Wire or delete the dead `IdleTimeout`. Effort S.
- H4. Re-enable gosec + a complexity linter (ratchet). Effort M.
- H5. Corrupt-JSON self-heal; `childEnvName` my-app/my_app collision; graceful Windows stop. Effort S–M.

**Phase 3 — Mature**
- M1. Cut the first `v0.x.0` tag + release automation; pin actions to SHAs (prereq: S1 green). Converts D→C on two dims. Effort S–M.
- M2. Freeze the flattened API surface for v0.x; curb churn. Effort M.
- M3. Docs: fix AttachCommands godoc (svc+autostart groups); fill FEATURES/BUGS/ISSUES stubs. Effort S.
- M4. Naming symmetry (`_other.go`→`_unix.go`); shared detach decl. Effort S.

## The one thing

**Fix the CI Go-version pin (`test.yml:18` + `build.yml:22`: `1.21` → `1.25`).** A two-file, S-effort
edit that turns the whole pipeline green on `main` — which then unlocks the release chain the
project has never had (green baseline → first `v0.x` tag → consumers can pin → deprecation policy
exercised). It is the root cause of BOTH D-grade dimensions. Nothing else in the route can be
trusted until the pipeline is green.

## Delta vs 2026-07-11 (hardening-loop rating)

- **Overall stage unchanged (4)**, but this independent run is more honest on CI: the prior rating
  scored Build/CI at stage 4 on the strength of *local* green lint; this run caught that **CI's Test
  job is RED on `main`** (local-green ≠ CI-green — the Go-version pin). CI/CD is really **D**.
- New defects the hardening checklist didn't capture: **dead `IdleTimeout`**, the **re-exec busy-loop**
  (`monitor.go:101`), unvalidated ports, and the **non-reachable transitive vuln**.
- Confirmed carried-forward strengths: Architecture/Correctness/Docs/Code-Quality all **A**;
  Security/Testing/Operational **B**; Release still gated on the never-cut first tag.

## Landed since this rating (/steps:next, 2026-07-12)

Most of the route was executed the same day:

- **S1** CI go-version drift → `go-version-file: go.mod` in both workflows (`e6a71cf`). *Green verification still needs a push.*
- **H1** CI OS matrix (win/macOS) + lint/vet gate (`e6a71cf`).
- **S4** re-exec busy-loop fixed → guard + backoff (`c1336aa`).
- **S3** port + service-name validation in `withDefaults`/`realOSService` (`faff04f`).
- **S2** cobra v1.10.2 + x/sys v0.44.0 → **GO-2026-5024 cleared**, govulncheck clean (`e4ffb74`).
- **H3** dead `IdleTimeout` removed (`973c991`).
- **H2** serverinfo error branches covered → pkg 67.5%, total 78.0% (`d8c39a5`).
- **H5** corrupt-JSON self-heal + `childEnvName` collision fixed (`7bb8627`). *Graceful Windows stop skipped — force-kill is correct for no-window detached processes.*

A second /steps:next cycle (2026-07-12) then cleared the rest of the actionable route:

- **H4** gosec + gocyclo re-enabled; real findings fixed (dir 0750, pid overflow guard),
  by-design G204 + test noise excluded (`af65e16`).
- **M3** AttachCommands/doc.go godoc corrected; FEATURES/BUGS filled (`c7e22f7`).
- **M4** `autostart_other.go` → `autostart_unix.go` (`eec4997`/`bb4f0d5`).
- **Security** `server.json` 0644 → 0600 (`3ed9565`).
- **Operational** `Options.OnRestart` metrics hook (`2ffb42d`).
- **Testing** real spawn→crash→restart integration test + TestMain hard timeout
  (`49c2d90`) → realSpawn 0%→81.8%, **total coverage 79.6%** (root 80.6%).

**Still open (all blocked on external prerequisites):** cut the first `v0.x` tag (needs a
green CI push — the release-chain payoff), the weaver/kody migration (external repos), and
the opt-in gRPC daemon path (needs the kody source + a design spec).

