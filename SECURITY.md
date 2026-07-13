# Security Policy

## Supported versions

This module is pre-1.0; only the latest `v0.x` release receives security fixes.

| Version | Supported |
|---------|-----------|
| latest `v0.x` | ✅ |
| older | ❌ |

## Reporting a vulnerability

Please report vulnerabilities **privately** — do not open a public issue for a
security problem.

- Preferred: GitHub's private vulnerability reporting on this repository
  (**Security → Report a vulnerability**), which opens a private advisory.

Include the affected version/commit, a description, and (ideally) a minimal
reproduction. You can expect an initial acknowledgement within a few days.
Once a fix is available it will ship in a new `v0.x` release with the advisory
published.

## Scope notes

This library spawns processes and performs privileged OS operations (service
install, Windows registry Run key, Task Scheduler). Relevant hardening already
in place:

- Every elevated write is gated by `RequirePrivilege` → `ErrNeedsPrivilege` (exit `5`).
- Subprocesses are launched as argument slices (never shell strings); the only
  commands run are the program's own executable and fixed OS utilities.
- Consumer-supplied ports and service names are validated before use.
- CI runs `gosec` and `govulncheck`.
