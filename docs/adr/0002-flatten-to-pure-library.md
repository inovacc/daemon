# ADR-0002: Flatten to a Pure-Library Module

## Status
Accepted (supersedes the structure and release decisions of
[ADR-0001](0001-project-scaffold.md)).

## Context
`daemon` exists solely to give consuming applications the ability to run as a
background/OS service (supervisor, `svc` install, elevation, Windows autostart).
It ships no product binary of its own — the reference `cmd/daemon` was only a demo.
The original scaffold nested the product under `pkg/daemon`, forcing consumers to
import the stuttering path `github.com/inovacc/daemon/pkg/daemon`, and carried a
GoReleaser binary-release pipeline that does not apply to a library.

## Decision
- **Flatten:** move `pkg/daemon/*` to the module root, so the product package is
  imported as `github.com/inovacc/daemon` (no stutter).
- **Hide internals:** move `pkg/serverinfo` to `internal/serverinfo` — it is an
  implementation detail (monitor-PID file), not part of the public surface.
- **Drop the binary:** remove `cmd/daemon` and the demo-only `internal/parameters`.
  The consumer wiring reference now lives in `example_test.go`
  (`ExampleAttachCommands`).
- **Drop binary-release infra:** remove `.goreleaser.yaml` and the GoReleaser
  release workflow. Releases are plain git tags consumed via `go get`.
- **Trim dependencies:** `go mod tidy` drops `inovacc/config`, `genversioninfo`,
  and their transitive trees; the module now needs only `spf13/cobra`,
  `kardianos/service`, and `golang.org/x/sys`.

## Consequences
### Positive
- Clean, non-stuttering import path; a single public package for consumers.
- Smaller dependency and maintenance surface; no release tooling to keep working.
- `internal/serverinfo` cannot be depended on (or broken) by downstream code.

### Negative
- No prebuilt binary or GoReleaser artifacts; anyone who wants a runnable daemon
  builds their own `main` (see `README.md` / `example_test.go`).
- Downstream imports of `.../pkg/daemon` must update to `.../daemon` (this module
  had no external consumers yet, so no deprecation window was needed).
