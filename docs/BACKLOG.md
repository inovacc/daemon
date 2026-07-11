# Backlog

## Priority Levels
| Priority | Timeline |
|----------|----------|
| P1 | This sprint |
| P2 | This quarter |
| P3 | Future |

## Items

- **Priority:** P2 — **Category:** Tech Debt — **Effort:** Medium
  - DEPRECATION: now the `daemon` module lands, migrate weaver + kody to consume it and mark their
    in-tree supervisor/serverinfo copies `Deprecated:` with a ≥30-day removal date.
- **Priority:** P3 — **Category:** Tech Debt — **Effort:** Small — **[DONE]**
  - ~~Decide fate of the `cmd/daemon` reference binary.~~ Resolved: dropped for a pure-library
    module (flattened `pkg/daemon`→root, `pkg/serverinfo`→`internal/serverinfo`, removed cmd +
    GoReleaser). Consumer wiring reference now lives in `example_test.go`. See ADR-0002.
