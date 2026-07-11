# Hardening Runbook — github.com/inovacc/daemon

Generated 2026-07-11. Pure-library module (no `cmd/`; consumers embed via `AttachCommands`).
Verified clean at baseline: `go vet ./...`; `go build` on windows/linux/darwin; `go test -race ./...` PASS.
Coverage **64.2%**. Current maturity **stage 3/5** → **target stage 4**.

Two items — **COV-01** and **ARCH-01** — are the ones that actually move a stage-3 dimension
(Testing, Architecture) to 4; everything else hardens or polishes around them.

## Scope & ground rules

- Do not build a shell command string anywhere — keep the arg-slice `exec.Command` pattern.
- Every elevated write stays gated by `RequirePrivilege`. Do not relax that.
- Preserve the one-way import graph and the hidden `internal/`; new seams follow the
  existing `<name>.go` declares `var`, `<name>_unix.go`/`_windows.go` implement pattern.
- Verify per item with its `verify_cmd`; run the full gate (`go vet`, 3-OS build,
  `go test -race`, `golangci-lint run`) at each phase boundary.

## Phase 1 — Stabilize (correctness / broken contract / security-critical)

Fix what is silently wrong or unreachable, and lock the most privilege-sensitive surface.

### H-01 · COV-01 (High, lev 5) — Windows autostart at 0% coverage
`autostart_windows.go:29`. The entire registry Run-key + `schtasks.exe` Task Scheduler
implementation (`Enable/Disable/Status/runKeyRoot/writeRunKey/deleteRunKey/runKeyStatus/
createTask/deleteTask/taskStatus/runSchtasks`) is untested — the most privilege-sensitive
code in the module (HKLM/HKCU writes, `/RU SYSTEM /RL HIGHEST` tasks).
- Extract a registry facade interface and a `schtasks`-runner func var (mirror the existing
  `newAutostartManager` seam) so tests assert the exact args passed to
  `/Create /TN /TR /RU SYSTEM /RL HIGHEST` and the HKLM-vs-HKCU hive selection without the real OS.
- Table-test `createTask` elevated/non-elevated arg construction and `runKeyRoot` hive choice.
- Unblocks SEC-01. Verify: `go test -run Autostart -cover ./...`. Moves Testing 3→4.
- **OUTCOME (2026-07-11, DONE):** Added a `runKeyStore` interface (`set/del/get`) with
  production `winRunKeyStore`, plus `runSchtasksFn` / `queryTaskFn` seams; the
  `windowsAutostart` methods now delegate to them. New `autostart_windows_test.go` (14
  table-tests) covers hive selection (HKCU/HKLM), `/RU SYSTEM /RL HIGHEST` arg construction,
  Enable/Disable/Status dispatch, unknown-method errors, and target building. Result:
  logic funcs `Enable/Disable/Status/runKeyRoot/writeRunKey/deleteRunKey/runKeyStatus/
  createTask/deleteTask/taskStatus` **0%→100%**, `realAutostartManager` 85.7%; only the thin
  OS wrappers (`winRunKeyStore.set/del/get`, `queryTask`, `runSchtasks`) remain 0% by design.
  Root pkg coverage **64.5%→72.1%**. Self-verified: build win/linux/darwin + `go vet` +
  `go test -race` all clean; manual diff review found no behavioral drift (Codex CLI returned
  no usable verdict this run). Branch `harden/cov-01-autostart-windows-seam`.

### H-02 · ARCH-01 (High, lev 4) — unreachable ports-forwarding contract
`options.go:43`. `Options.portsExplicit` is package-private with no exported setter, so
`args.go buildMonitorArgs` never forwards `--port/--grpc-port` for any external caller;
`daemon.Start` / `service start` silently revert to compiled-in defaults.
- Derive `portsExplicit` inside `withDefaults()` (non-default HTTPPort/GRPCPort ⇒ explicit),
  OR expose `Options.WithPorts(http, grpc)`.
- Add a test asserting `buildMonitorArgs` forwards the flags for a consumer-set port.
- Verify: `go test ./... -run TestBuildMonitorArgs`. Moves Architecture 3→4.
- **OUTCOME (2026-07-11, DONE):** Derived `portsExplicit` inside `withDefaults()` from a
  **non-default** port (`(HTTPPort!=0 && !=DefaultHTTPPort) || (GRPCPort!=0 && !=DefaultGRPCPort)`),
  set before defaults are filled and only ever OR'd true. Chose "non-default" over "non-zero"
  because `withDefaults()` runs twice in the real flow (`AttachCommands` cobra.go:27 → `Start`
  daemonize.go:44); a non-zero rule would spuriously flip the flag on the second pass over the
  already-defaulted 9500/9501 ports and forward defaults for every consumer. Added 3 tests
  (args_test.go): consumer-set ports forwarded, single-port override forwards both, and a
  double-`withDefaults` idempotency regression guard. Result: `buildMonitorArgs` **100%**,
  `withDefaults` 95.5%; consumer-set ports now reach the worker (contract was previously dead
  code). Self-verified: build win/linux/darwin + `go vet` + `go test -race` clean, no drift
  (existing arg tests unchanged); Codex CLI again returned no usable verdict (0 tool_uses).
  Branch `harden/cov-01-autostart-windows-seam`.

### H-03 · ERR-01 (Med, lev 3) — silent success on health-wait timeout
`daemonize.go:68`. The TOCTOU health-wait loop returns `(pid, nil)` on 5s timeout without
signalling that `serverinfo` was never observed; `startCommand` prints "started" for any nil
error, so a spawned-but-crashed monitor reports success.
- Return a distinguishable sentinel (`ErrHealthCheckTimeout`) or a status flag alongside pid;
  have `startCommand` differentiate confirmed vs unconfirmed.
- Verify: `go test ./... -run TestStart`.
- **OUTCOME (2026-07-11, DONE):** Added the `ErrHealthCheckTimeout` sentinel; `Start` now
  returns `(pid, ErrHealthCheckTimeout)` when the monitor never writes serverinfo within the
  wait window, instead of the old silent `(pid, nil)` — a spawned-but-crashed monitor no longer
  masquerades as success. `startCommand` handles the sentinel with an explicit
  `started: pid=N (unconfirmed: monitor did not report ready in time; run 'status' to verify)`
  line and exit 0 — differentiating confirmed from unconfirmed without false-alarming a merely
  slow monitor. Introduced a `healthWaitTimeout` package-var seam (prod 5s) so tests exercise
  the timeout path in ~50ms; added `TestStartHealthCheckTimeout` (sentinel + pid returned) and
  `TestStartCommandReportsUnconfirmed` (CLI message). Coverage: `Start` 90%, `startCommand`
  11%→50%, total 71.3%→72.2%. The `Start` return-value change is a correctness fix (signature
  unchanged, pid still returned; the old nil-on-timeout was the defect), not an API break.
  Self-verified: build win/linux/darwin + `go vet` + `go test -race` clean, no regression
  (existing Start/Stop tests unchanged); Codex unavailable this session (0 tool_uses on both
  dispatches). Branch `harden/cov-01-autostart-windows-seam`.

### H-04 · ERR-03 (Med, lev 3) — swallowed taskkill diagnostics (Windows)
`stop_windows.go:12`. `stopProcess` uses `.Run()` and discards taskkill stdout/stderr, so
"Access is denied" never reaches the wrapped error — inconsistent with `runSchtasks`.
- Switch to `CombinedOutput()` and fold trimmed output into the returned error (mirror `runSchtasks`).
- Verify: `go build ./... ; go vet ./...`.
- **OUTCOME (2026-07-11, DONE):** `stopProcess` (stop_windows.go) now runs `taskkill /T /F` via
  `CombinedOutput()` and, on non-zero exit, returns
  `fmt.Errorf("taskkill pid %d: %w: %s", pid, err, strings.TrimSpace(out))` — folding the OS
  message (e.g. "Access is denied" without admin) into a `%w`-wrapped error, exactly mirroring
  `runSchtasks`. `Stop()` wraps that once more (`daemon: stop pid N: taskkill …`). Success path
  unchanged. No unit test added — it is a Windows-only destructive exec leaf (same by-design
  0% class as `runSchtasks`/`queryTask`/`winRunKeyStore`); verified by build+vet+race+3-OS and
  diff review against the `runSchtasks` template. `TestStopCallsPlatformStop` (fakes
  `stopProcessFn`) is unaffected. Codex unavailable this session (0 tool_uses). Branch
  `harden/cov-01-autostart-windows-seam`.

**Phase gate:** `go vet` + 3-OS build + `go test -race` + the two new tests green.
**Phase 1 COMPLETE (2026-07-11):** H-01..H-04 all landed; gate green (vet, 3-OS build, race PASS).

## Phase 2 — Harden (robustness / reuse / deps / lint gate)

### H-05 · ERR-02 (Med, lev 2) — swallowed process-group kill error
`stop_unix.go:11`. Discards the group-kill error before falling back to single-pid kill,
losing EPERM-vs-ESRCH diagnostics. Fix: `errors.Join(groupErr, singleErr)` when both fail.
Verify: `GOOS=linux go build ./... ; go vet ./...`.

### H-06 · SEC-01 (Low, lev 3) — validate consumer-controlled ServiceName/target
`autostart_windows.go:140`. Not a live vuln (arg-slice exec, no shell, elevation gated), but
add defense-in-depth: validate `ServiceName` against a conservative charset before use as a
`/TN` name and registry value, reject empty/whitespace. Depends on H-01's test seam.
Verify: `go test -run Autostart ./...`.

### H-07 · CON-01 (Med, lev 3) — document Stop() bounded-wait leak tradeoff
`svc.go:73`. `Stop()` has no hard join if a caller-supplied run func ignores ctx; it times
out and returns while the goroutine leaks. Document/enforce that `run` must be ctx-responsive;
note the timeout-then-leak tradeoff in Stop's doc comment. Verify: `go test -race ./... -run TestProgram`.
Moves Concurrency doc-completeness within stage 4.

### H-08 · LINT-01 (Low, lev 2) — predeclared `any` shadow
`autostart.go:182`. Rename local `any` → `found`/`anyEnabled`. Verify: `golangci-lint run ./...`.

### H-09 · LINT-02 (Low, lev 1) — 6 wsl_v5 whitespace issues
`autostart_windows.go:91,105,127` + `svc_test.go:223,248,273`. Insert blank line before each
flagged defer/statement (or scope out wsl_v5 in `.golangci.yml`). Verify: `golangci-lint run ./...`.

### H-10 · ERR-04 (Low, lev 2) — status queries hide real errors
`autostart_windows.go:129`. `runKeyStatus`/`taskStatus` treat any query error as "not enabled".
Distinguish `registry.ErrNotExist` / schtasks "not found" from other errors; surface unexpected
errors via `Status()`'s error return. Verify: `go build ./... ; go vet ./...`.

### H-11 · DUP-01 (Low, lev 1) — duplicated detached SysProcAttr
`reexec_windows.go:29` vs `spawn_windows.go:18`. Extract `detachedSysProcAttr()` helper in a
windows-tagged file, reuse in both. Verify: `go build ./... && go vet ./...`.

### H-12 · DUP-02 (Low, lev 1) — repeated registry open+defer-close prologue
`autostart_windows.go:86`. Optional `withRunKey(elevated, access, fn)` helper. Low priority.
Verify: `go build ./...`.

### H-13 · CON-02 (Low, lev 1) — unguarded double Start()
`svc.go:52`. A second `Start()` overwrites `p.cancel/p.done`. Guard with a started flag / doc
the single-call invariant. Verify: `go test ./... -run TestProgram`.

### H-14 · CON-03 (Low, lev 1) — restartGuard not concurrency-safe
`restartguard.go:1`. Add a doc comment that the type is single-goroutine only; no code change.
Verify: `go test -race ./...`.

### H-15 · DEP-01 (Low, lev 1) — dependency justification
`go.mod:5`. No action: cobra, kardianos/service, x/sys all justified, none stdlib-replaceable.
Record the rationale. Verify: `go mod why github.com/kardianos/service`.

**Phase gate:** `golangci-lint run ./...` exits 0; full gate green.
**Phase 2 COMPLETE (2026-07-11):** H-05, H-06, H-07, H-08, H-09, H-10, H-11, H-13, H-14, H-15
landed; H-12 skipped (optional — `withRunKey` doesn't fit `get`'s ErrNotExist branch). Gate:
`golangci-lint run ./...` **0 issues**, `go vet` clean, `go test -race` PASS, 3-OS build clean.
Build/CI dimension → stage 4 (lint green); SEC-01 validation, CON-01/02/03 concurrency docs, and
ERR-02/04 diagnostics all in. Commits fd5e63f, 014b3b5, 7b04772, 5c526d4, e897165, 80e3fbd, 5eaad01.

## Phase 3 — Mature (coverage / docs / polish)

### H-16 · COV-02 (Med, lev 3) — daemonize command handlers barely tested
`cobra.go:82` (`startCommand` 11.1%, `stopCommand` 20.0%, `statusCommand` 14.3%). Drive commands
through cobra with a temp DataDir + fake Serve/Store to cover running/not-running and
ErrAlreadyRunning/ErrNotRunning exit-code mapping. Verify: `go test -run Cobra -cover ./...`.

### H-17 · COV-03 (Med, lev 3) — supervisor entry + exec wrappers 0%
`monitor.go:45` (`RunMonitor`/`realSpawn` 0%). Add a direct `RunMonitor` test with a cancelled
context and a fake `spawnFn`; accept 0% on irreducible syscall leaves. Verify: `go test -run Monitor -cover ./...`.

### H-18 · COV-04 (Low, lev 2) — svc path gaps
`svc.go:96` (`realOSService` 28.6%, `svcStatusCommand` 69.2%). Cover the ServiceName-empty guard
and status running/stopped/not-installed branches via the existing osService fake.
Verify: `go test -run Svc -cover ./...`.

### H-19 · ERR-05 (Low, lev 2) — non-idempotent stop
`cobra.go:104`. Add `if errors.Is(err, ErrNotRunning) { print friendly; return nil }` to
`stopCommand` for symmetry with `startCommand`. Verify: `go test ./... -run TestStop`.

### H-20 · ERR-06 (Low, lev 1) — swallowed server.json removal error
`daemonize.go:94`. Log the `store.Remove()` error as a non-fatal warning instead of discarding.
Verify: `go vet ./...`. Moves Observability toward 4.

### H-21 · ARCH-02 (Low, lev 2) — reexecFn seam misplaced
`monitor.go:31`. Move `var reexecFn = reexecSelf` into a dedicated `reexec.go` so all four
platform seams follow one pattern. Verify: `go build ./...`.

### H-22 · DX-01 (Low, lev 2) — optional autostart Example
`example_test.go:1`. DX already Mature; optionally add a runnable Example for the autostart group.
Verify: `go test -run Example ./...`.

**Phase gate:** coverage improved above 64.2% baseline (COV-01/02/03/04); full gate green.

## Exit bar (stage 4)

- COV-01 + ARCH-01 landed → Testing and Architecture dimensions at stage 4.
- ERR-01 + ERR-03 landed → Error-handling no longer reports silent success / swallows kill diagnostics.
- `golangci-lint run ./...` exits 0 (CI green) → Build/CI dimension at stage 4.
- `go vet` clean, 3-OS build clean, `go test -race` PASS, coverage above 64.2%.

## Phase 3 COMPLETE (2026-07-11)

All Phase-3 (Mature) items landed on `harden/cov-01-autostart-windows-seam`:

- **H-16** (COV-02) `6b6dad1` — start/stop/status command handlers 0%→100%.
- **H-17** (COV-03) `3b73426` — RunMonitor entry + both context-cancellation branches.
- **H-18** (COV-04) `2711d6f` — realOSService success path + every svcStatus label.
- **H-19** (ERR-05) `e7afc33` — stop idempotent on ErrNotRunning (exit 0).
- **H-20** (ERR-06) `406ed72` — log server.json removal failure → **Observability stage 4**.
- **H-21** (ARCH-02) `2e9340d` — reexecFn seam relocated to reexec.go.
- **H-22** (DX-01) — runnable autostart Example.

Coverage climbed 72.0% → **78.0%** (root pkg) across COV-02/03/04. Full gate green at
each commit: golangci-lint 0 issues, go vet clean, go test -race PASS, windows/linux/darwin
build clean. Codex verifier remained non-functional (0 tool_uses); every item self-verified
adversarially (build/vet/lint/race/3-OS + diff review).

**All 22 checklist items now checked (H-12 skipped-optional).** Every phase complete.
