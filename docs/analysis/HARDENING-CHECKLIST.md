# Hardening Checklist — github.com/inovacc/daemon

Generated 2026-07-11. 22 items · stage 3 → target 4. Ordered by phase, then leverage.
Legend: `[ ]` unchecked · sev = severity · lev = leverage. See HARDENING-RUNBOOK.md for detail.

## Phase 1 — Stabilize

- [x] **H-01** · COV-01 · coverage · sev High · lev 5 · `autostart_windows.go:29`
  - blocks: — · unblocks: SEC-01 · verify: `go test -run Autostart -cover ./...`
  - Seam registry+schtasks; table-test elevation/hive branches. Moves Testing 3→4.
  - DONE 2026-07-11: runKeyStore/runSchtasksFn/queryTaskFn seams + 14 table-tests. autostart_windows.go logic funcs 0%→100% (only thin OS wrappers remain 0%); root pkg 64.5%→72.1%. Build 3-OS + vet + race clean, no behavior drift. Commit on harden/cov-01-autostart-windows-seam.
- [x] **H-02** · ARCH-01 · architecture · sev High · lev 4 · `options.go:43`
  - blocks: — · unblocks: — · verify: `go test ./... -run TestMonitorArgs`
  - Make portsExplicit reachable (WithPorts / derive). Moves Architecture 3→4.
  - DONE 2026-07-11: derive portsExplicit in withDefaults() from a NON-DEFAULT port (idempotent across the double-withDefaults real flow; "non-zero" would spuriously flip on the 2nd pass). +3 tests (consumer-set, single-port override, double-withDefaults guard). buildMonitorArgs 100%. Build 3-OS + vet + race clean; no drift. Commit on harden/cov-01-autostart-windows-seam.
- [x] **H-03** · ERR-01 · error-handling · sev Med · lev 3 · `daemonize.go:68`
  - blocks: — · unblocks: — · verify: `go test ./... -run TestStart`
  - Distinguish spawned-but-unconfirmed from success (ErrHealthCheckTimeout).
  - DONE 2026-07-11: added ErrHealthCheckTimeout; Start returns (pid, sentinel) on health-wait timeout instead of (pid, nil). startCommand prints an explicit "unconfirmed… run 'status'" line (exit 0, no false-alarm). Added healthWaitTimeout seam + 2 tests (Start sentinel, startCommand message). Start 90%, startCommand 11→50%. Build 3-OS + vet + race clean. Commit on harden/cov-01-autostart-windows-seam.
- [x] **H-04** · ERR-03 · error-handling · sev Med · lev 3 · `stop_windows.go:12`
  - blocks: — · unblocks: — · verify: `go build ./... ; go vet ./...`
  - Use CombinedOutput; fold taskkill stderr into the error.
  - DONE 2026-07-11: stopProcess now uses CombinedOutput and folds trimmed taskkill output into a %w-wrapped error (mirrors runSchtasks), so "Access is denied" reaches the caller instead of a bare exit status. Build 3-OS + vet + race clean; no behavior drift (exec leaf, TestStopCallsPlatformStop unaffected). Commit on harden/cov-01-autostart-windows-seam.

## Phase 2 — Harden

- [x] **H-05** · ERR-02 · error-handling · sev Med · lev 2 · `stop_unix.go:11`
  - blocks: — · unblocks: — · verify: `GOOS=linux go build ./... ; go vet ./...`
  - errors.Join group-kill + single-pid kill errors.
  - DONE 2026-07-11: errors.Join(groupErr, singleErr) when both kills fail. linux/darwin build+vet clean. Commit 5eaad01.
- [x] **H-06** · SEC-01 · security · sev Low · lev 3 · `autostart_windows.go:140`
  - blocks: COV-01 (do after H-01) · unblocks: — · verify: `go test -run Autostart ./...`
  - Validate ServiceName charset; keep arg-slice exec.
  - DONE 2026-07-11: validateServiceName (letters/digits/-_. only, reject empty) called in realAutostartManager; cross-platform table test. Commit fd5e63f.
- [x] **H-07** · CON-01 · concurrency · sev Med · lev 3 · `svc.go:73`
  - blocks: — · unblocks: — · verify: `go test -race ./... -run TestProgram`
  - Doc Stop() bounded-wait leak tradeoff; require ctx-responsive run.
  - DONE 2026-07-11: documented bounded-wait-vs-leak tradeoff + ctx-responsive Serve requirement on Stop(). Commit 014b3b5.
- [x] **H-08** · LINT-01 · lint · sev Low · lev 2 · `autostart.go:182`
  - blocks: — · unblocks: — · verify: `golangci-lint run ./...`
  - Rename local `any` → found.
  - DONE 2026-07-11: renamed `any` → `anyEnabled`. golangci-lint green. Commit 7b04772.
- [x] **H-09** · LINT-02 · lint · sev Low · lev 1 · `autostart_windows.go:91`
  - blocks: — · unblocks: — · verify: `golangci-lint run ./...`
  - Fix 6 wsl_v5 whitespace issues.
  - DONE 2026-07-11: inserted wsl_v5 blank lines in winRunKeyStore + test seams. golangci-lint 0 issues → Build/CI dim stage 4. Commit 7b04772.
- [x] **H-10** · ERR-04 · error-handling · sev Low · lev 2 · `autostart_windows.go:129`
  - blocks: — · unblocks: — · verify: `go build ./... ; go vet ./...`
  - Distinguish not-exist from real query errors in status.
  - DONE 2026-07-11: get→(value,present,err); queryTask→(bool,err) via *exec.ExitError; Status surfaces unexpected errors. +2 tests. Commit 5c526d4.
- [x] **H-11** · DUP-01 · duplication · sev Low · lev 1 · `reexec_windows.go:29`
  - blocks: — · unblocks: — · verify: `go build ./... && go vet ./...`
  - Extract detachedSysProcAttr() helper.
  - DONE 2026-07-11: detach_windows.go detachedSysProcAttr() reused by spawnDetached + reexecSelf. Commit e897165.
- [~] **H-12** · DUP-02 · duplication · sev Low · lev 1 · `autostart_windows.go:86`
  - blocks: — · unblocks: — · verify: `go build ./...`
  - Optional withRunKey() helper (low priority).
  - SKIPPED 2026-07-11 (optional): get()'s ErrNotExist-on-open special case (H-10) does not fit an error-only helper; extracting only set/del would split the pattern rather than unify it. Not worth the churn.
- [x] **H-13** · CON-02 · concurrency · sev Low · lev 1 · `svc.go:52`
  - blocks: — · unblocks: — · verify: `go test ./... -run TestProgram`
  - Guard/doc double-Start invariant.
  - DONE 2026-07-11: documented single-call invariant on program.Start. Commit 80e3fbd.
- [x] **H-14** · CON-03 · concurrency · sev Low · lev 1 · `restartguard.go:1`
  - blocks: — · unblocks: — · verify: `go test -race ./...`
  - Doc restartGuard is single-goroutine only.
  - DONE 2026-07-11: documented restartGuard is single-goroutine-only. Commit 80e3fbd.
- [x] **H-15** · DEP-01 · deps · sev Low · lev 1 · `go.mod:5`
  - blocks: — · unblocks: — · verify: `go mod why github.com/kardianos/service`
  - No action; record dependency justification.
  - DONE 2026-07-11: `go mod why` confirms cobra, kardianos/service, x/sys all directly used; none stdlib-replaceable. No code change. Rationale recorded in AGENTS.md deps + runbook.

## Phase 3 — Mature

- [x] **H-16** · COV-02 · coverage · sev Med · lev 3 · `cobra.go:82`
  - blocks: — · unblocks: — · verify: `go test -run Cobra -cover ./...`
  - Cover start/stop/status command branches + exit-code mapping.
  - DONE 2026-07-11: added runSubcommand harness + 8 branch tests (start already-running/success/error-propagation, stop success/not-running-propagation, status running/not-running). startCommand/stopCommand/statusCommand 0%→100%; root pkg 72.0%→75.3%. Exit-code mapping already covered by exitstatus_test. Race + 3-OS build + lint(0) clean. Commit on harden/cov-01-autostart-windows-seam.
- [x] **H-17** · COV-03 · coverage · sev Med · lev 3 · `monitor.go:45`
  - blocks: — · unblocks: — · verify: `go test -run Monitor -cover ./...`
  - Direct RunMonitor test with cancelled ctx + fake spawnFn.
  - DONE 2026-07-11: +2 tests — RunMonitor(pre-cancelled ctx) covers the public entry + top-of-loop cancellation + serverinfo write/defer-remove; m.run with a spawn that cancels covers handleCrash's ctx.Done() branch. RunMonitor 0%→100%, handleCrash 100%; root pkg 75.3%→76.3%. (realSpawn stays 0% — thin os/exec leaf.) Race + 3-OS build + lint(0) clean. Commit on harden/cov-01-autostart-windows-seam.
- [x] **H-18** · COV-04 · coverage · sev Low · lev 2 · `svc.go:96`
  - blocks: — · unblocks: — · verify: `go test -run Svc -cover ./...`
  - Cover realOSService guard + svcStatusCommand branches.
  - DONE 2026-07-11: +TestRealOSServiceBuildsService (success path past the empty-name guard) + TestSvcStatusLabels (table over not-installed/running/stopped/unknown — every status→label switch branch). realOSService 42%→85.7%, svcStatusCommand →92.3%; root pkg 76.3%→77.6%. Race + 3-OS build + lint(0) clean. Commit on harden/cov-01-autostart-windows-seam.
- [x] **H-19** · ERR-05 · error-handling · sev Low · lev 2 · `cobra.go:104`
  - blocks: — · unblocks: — · verify: `go test ./... -run TestStop`
  - Make stop idempotent on ErrNotRunning.
  - DONE 2026-07-11: stopCommand now maps ErrNotRunning to a benign "not running" / exit-0 (mirrors start's ErrAlreadyRunning), so repeated stop / stop-after-crash no longer errors. Stop() itself still returns the sentinel (unchanged API). Repurposed the H-16 not-running test to assert idempotency + added a real stopProcessFn-failure test so genuine stop errors still propagate. stopCommand 100%; root pkg 77.6%→78.0%. Race + 3-OS build + lint(0) clean. Commit on harden/cov-01-autostart-windows-seam.
- [ ] **H-20** · ERR-06 · error-handling · sev Low · lev 1 · `daemonize.go:94`
  - blocks: — · unblocks: — · verify: `go vet ./...`
  - Log store.Remove() error as non-fatal warning.
- [ ] **H-21** · ARCH-02 · architecture · sev Low · lev 2 · `monitor.go:31`
  - blocks: — · unblocks: — · verify: `go build ./...`
  - Move reexecFn into dedicated reexec.go.
- [ ] **H-22** · DX-01 · dx · sev Low · lev 2 · `example_test.go:1`
  - blocks: — · unblocks: — · verify: `go test -run Example ./...`
  - Optional autostart Example.
