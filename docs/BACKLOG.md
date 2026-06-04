# Backlog

## Priority Levels
| Priority | Timeline |
|----------|----------|
| P1 | This sprint |
| P2 | This quarter |
| P3 | Future |

## Items

- **Priority:** P2 — **Category:** Tech Debt — **Effort:** Medium
  - DEPRECATION: once `pkg/daemon` lands, migrate weaver + kody to consume it and mark their
    in-tree supervisor/serverinfo copies `Deprecated:` with a ≥30-day removal date.
- **Priority:** P3 — **Category:** Feature — **Effort:** Small
  - Decide fate of generated `cmd/daemon` reference binary (keep as example vs drop for pure-library).
