# AGENTS.md
<!-- rev:002 -->

Canonical cross-tool agent instructions for `github.com/inovacc/daemon` (read by Claude
Code via `CLAUDE.md`'s `@AGENTS.md` import, plus Codex, Cursor, Gemini). Keep it lean;
link deep detail to `docs/*.md`.

## What this module is

A reusable **Go library** (no binary) that gives a consumer application the ability to run
as a long-lived background service: a monitor→worker supervisor with a fork-loop guard,
OS-service registration (kardianos/service), privilege gating, and Windows launch-at-logon
(registry Run key / Task Scheduler). Consumers wire it in with a single call —
`daemon.AttachCommands(root, daemon.Options{...})` — and ship their own executable.

- Package `daemon` at the module **root** (flattened — see ADR-0002). No `cmd/`.
- Hidden `internal/serverinfo` holds the monitor-PID file (write/read/IsRunning + stale self-heal).
- Architecture + lifecycle: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), [docs/SERVICE_LIFECYCLE.md](docs/SERVICE_LIFECYCLE.md).
- Windows autostart: [docs/AUTOSTART.md](docs/AUTOSTART.md).

## Build / test / lint (prefer `task`)

| Task | Command | Raw equivalent |
|------|---------|----------------|
| Test (race + coverage) | `task test` | `go test -race -coverprofile=coverage.out ./...` |
| Coverage % | `task test:cover` | `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out` |
| Format | `task fmt` | `go fmt ./... && goimports -w .` |
| Vet | `task vet` | `go vet ./...` |
| Lint | `task lint` | `golangci-lint run ./...` |
| Build (type-check) | `task build` | `go build ./...` |
| Everything | `task check` | fmt + vet + lint + build + test |
| Deps | `task deps` | `go mod download && go mod tidy && go mod verify` |

Run `task check` before every commit. The module cross-compiles for windows/linux/darwin —
verify platform-tagged files with `GOOS=<os> go build ./...`.

## Code style

- Idiomatic Go; `gofmt`/`goimports` clean. Wrap errors with `%w`; export sentinels
  (`ErrAlreadyRunning`, `ErrNotRunning`, `ErrHealthCheckTimeout`, `ErrNeedsPrivilege`).
- **Platform split** by build tags: `<name>_windows.go` / `_unix.go` (`!windows`). Keep the
  shared declaration in `<name>.go`.
- **Test seams** are package-var func indirection (`spawnDetachedFn`, `stopProcessFn`,
  `reexecFn`, `newOSService`, `newAutostartManager`, `runKeys`, `runSchtasksFn`, `queryTaskFn`,
  `isElevatedFn`, `healthWaitTimeout`). Override in tests, restore via `t.Cleanup`; never seam
  in a way that changes production behavior.
- Build a command as an **arg slice** for `exec.Command` — never a shell string. Fold a
  failed command's output into the returned error (see `runSchtasks` / `stopProcess`).
- The monitor never carries the worker role or ports (`buildMonitorArgs`); the worker always
  carries both ports for `ps`/Task Manager visibility (`buildWorkerArgs`). This seam is
  bug-prone — keep it covered.

## Security

- Every elevated write (HKLM Run key, `schtasks /RU SYSTEM /RL HIGHEST`, OS-service install)
  stays gated by `RequirePrivilege` → `ErrNeedsPrivilege` → exit `5` (`ExitNeedsPrivilege`).
  Do not relax that gate.
- No secrets in the repo; BSD-3 licensed. Validate consumer-supplied names before using them
  as task/registry-value names.

## Commits / PRs

- Conventional commits (`feat:`, `fix:`, `test:`, `docs:`, `refactor:`). No AI attribution.
- One logical change per commit; keep the tree green (`task check`) at each commit.
- Bump `<!-- rev:NNN -->` on any living doc you edit (README/AGENTS/CLAUDE/architecture/roadmap);
  do not stamp dated records (ADRs, specs, hardening runbook).
