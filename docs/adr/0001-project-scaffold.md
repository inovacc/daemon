# ADR-0001: Project Scaffold and Tooling Choices

## Status
Accepted

## Context
`daemon` is a reusable Go module consolidating the daemon/service layers duplicated across
`acer/projects` (notably weaver's supervisor and kody's gRPC daemon). It needs a standard
structure, tooling, and conventions.

## Decision
- **Shape:** Library-first (`pkg/` is the product); `cmd/daemon` retained as a reference example.
- **Service pattern:** kardianos/service (`--service`) + `inovacc/config` for OS-service install.
- **Structure:** cmd/, internal/, pkg/ (Hexagonal/Clean).
- **CLI Framework:** Cobra via `omni scaffold cobra init`.
- **Task Runner:** Taskfile. **Linting:** golangci-lint v2. **Releases:** GoReleaser.
- **Module Path:** github.com/inovacc/daemon
- **License:** BSD 3-Clause (per the owner's standing convention; overrides the scaffold MIT default).

## Consequences
### Positive
- Consistent structure/tooling; automated release + CI from day one.
- Library shape keeps the daemon layer reusable across projects.
### Negative
- Requires external tools (golangci-lint, goreleaser, task).
- `--service` generates app-shaped `internal/` code that must be re-surfaced under `pkg/` for reuse.
