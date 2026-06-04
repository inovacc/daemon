# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`daemon` (planned module path `github.com/inovacc/daemon`) is a **reusable Go module** that
consolidates the daemon/service-layer code currently duplicated across `acer/projects`. It
unifies two production patterns into one option-driven API:

- **Supervisor layer** — monitor/inner-process restart loop, signal handling, crash
  detection, self-upgrade, platform service install. Source of truth: `dyammarcano/weaver`
  (`cmd/weaver/{monitor,serve,server,server_unix,server_windows}.go`, `internal/serverinfo`).
- **gRPC thin-client daemon layer** — gRPC server + health, idle auto-shutdown, server
  discovery + auto-start. Source of truth: `inovacc/kody` (`internal/server/grpc/{server,idle}.go`).

> **Status: scaffolded shell, API unimplemented.** Module `github.com/inovacc/daemon` is
> initialized (Cobra/Taskfile/goreleaser/CI, BSD-3, `kardianos/service` + `inovacc/config`).
> The reusable API lives under `pkg/daemon` (currently `doc.go` + `exitstatus.go` only).
> `cmd/daemon` and `internal/service` are the omni-generated reference example — the
> **product is the `pkg/` library** (shape decision: library-only). `docs/SERVICE_LIFECYCLE.md`
> is the authoritative architecture spec; the patterns are encoded in the `thin-client-daemon`
> and `supervisor-pattern` skills. Implementation order: see `docs/ROADMAP.md`.

## Architecture (read `docs/SERVICE_LIFECYCLE.md` first)

The whole point of the module is to hide a set of **bug-prone seams** so consumers can't get
them wrong. The non-obvious invariants — each one a hard-won lesson from weaver/kody:

- **Three process roles, one binary.** User-facing verbs are public Cobra commands; the two
  supervisor roles are **hidden Cobra commands prefixed `__`**: `__monitor` (the restart
  loop) and `__worker` (the inner process). Ports are passed as visible args on `__worker`
  (`--port`/`--grpc-port`) so role+port show up in `ps`/Task Manager.
- **Spawn chain:** `service start` → daemonize spawns `__monitor` (detached) → monitor spawns
  `__worker --port N --grpc-port N`. `daemonize()` MUST build `__monitor` args, never
  `__worker` — if it spawns the worker directly, the monitor layer is skipped, `serverinfo`
  is never written, and `stop`/`status` silently break.
- **`serverinfo` stores the monitor PID** (not the worker), so `stop` kills the whole tree
  from the root (`taskkill /T /F` on Windows, `SIGINT` on Unix — split into platform files
  with build tags; never use `syscall.SIGTERM` in cross-platform code).
- **`__worker` must skip the singleton `IsRunning()` check.** The monitor already wrote
  `serverinfo` with its own PID; otherwise the worker sees "already running" and exits.
- **Deferred init.** `New()` is lightweight (config/logger/idle only). Open DB and register
  gRPC services in `initServices()` *after* the port bind succeeds — eager init causes
  `SQLITE_BUSY` when instances race.
- **Everything lives in the data dir** (`%LOCALAPPDATA%\App` / `~/.local/share/App` /
  `~/Library/Application Support/App`), never CWD: config.yaml, db, lock, server.json, panic logs.

### Fork/spawn loop-hell guard (non-negotiable — enforce in the module, not the consumer)
- Sliding window: 4 restarts within 60s → abort the monitor instead of restarting.
- Exponential backoff between restarts: `min(1s * 2^attempt, 60s)`; reset after a healthy run.
- `ExitRestart` (exit code 3) uses `continue` to **bypass** the crash counter and backoff —
  intentional/API-driven restarts must not trip the loop detector.
- Self-spawn env guard (`<APP>_DAEMON_CHILD`) + strip legacy supervisor env prefixes from the
  child env. TOCTOU: re-acquire the single-instance lock right before spawning; the post-spawn
  wait loop checks **health**, not just `server.json` existence.

### Exit-code protocol
`ExitSuccess=0` (clean → monitor exits) · `ExitError=1` (crash → restart) ·
`ExitRestart=3` (intentional restart → re-loop, no crash count) · `ExitUpgrade=4` (binary
replaced → monitor re-exec's itself; `syscall.Exec` on Unix, spawn on Windows).

## Commands

Taskfile is the runner (prefer `task` over raw go): `task build | run | test | fmt | vet |
lint | check | deps | clean | install | release`. `task check` (fmt+vet+lint+test) before any PR.

- Build / test everything: `go build ./...` · `go test ./...`
- Run a single test: `go test ./pkg/daemon/ -run TestName -v`
- Lint: `golangci-lint run --fix ./... --timeout=5m` (v2 config in `.golangci.yml`)
- Reference binary (example only, not the product): `go run ./cmd/daemon`
- Follow global Go standards (`~/.claude/CLAUDE.md`): `go run` over build-then-run, table-driven
  tests, 80%+ coverage, `log/slog` structured logging.
- Supervisor/server tests need a `TestMain` hard timeout (120s) and a `.scripts/guard-instances.sh`
  cleanup script — without them, crashed tests leave accumulating `*.test.exe` zombies on Windows.
- `GracefulStop()` must always have a 10s timeout fallback to `Stop()`, and every `exec.Command`
  must be `exec.CommandContext` with a timeout.

## Conventions

- Breaking/invasive changes follow the deprecation strategy in the global `~/.claude/CLAUDE.md`
  (add-alongside, mark `Deprecated:` with a ≥30-day removal date, log usage, track in BACKLOG).
- When porting weaver/kody onto this module, keep the old code behind a deprecation date rather
  than deleting in-place.
- License: BSD 3-Clause for this new module.

---

# context-mode — MANDATORY routing rules

You have context-mode MCP tools available. These rules are NOT optional — they protect your
context window from flooding. A single unrouted command can dump 56 KB into context.

- **curl / wget / inline HTTP / WebFetch are BLOCKED.** Use `ctx_fetch_and_index(url, source)`
  then `ctx_search(queries)`, or `ctx_execute(language, code)` for HTTP in the sandbox.
- **Bash is only for** `git`, `mkdir`, `rm`, `mv`, `cd`, `ls`, `npm/pip install`, and other
  short-output commands. For anything with >20 lines of output use
  `ctx_batch_execute(commands, queries)` or `ctx_execute(language, code)`.
- **Read** is correct when you intend to **Edit** a file; use `ctx_execute_file(path, language, code)`
  when reading to analyze/explore/summarize. Run large **Grep** searches via `ctx_execute` shell.
- Tool hierarchy: `ctx_batch_execute` (primary) → `ctx_search` → `ctx_execute`/`ctx_execute_file`
  → `ctx_fetch_and_index` → `ctx_index`.
- Keep responses under 500 words. Write artifacts to FILES; return only path + 1-line description.
- `ctx stats` / `ctx doctor` / `ctx upgrade` map to the `ctx_stats` / `ctx_doctor` / `ctx_upgrade` tools.
