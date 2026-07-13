## What

<!-- One-line summary of the change. -->

## Why

<!-- Link the issue or backlog item this addresses. -->

## Checklist

- [ ] `task check` passes (fmt, vet, lint, build, race tests)
- [ ] Cross-compiles: `GOOS=windows|linux|darwin go build ./...`
- [ ] Lint is clean on **each** OS — build tags hide `_unix.go` / `_windows.go` files from a
      single-platform run (`GOOS=<os> golangci-lint run ./...`)
- [ ] Tests added/updated; coverage stays at or above 80%
- [ ] Conventional commit messages (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `ci:`)
- [ ] Docs updated (README / AGENTS.md / `docs/*`) if behavior or the public API changed
- [ ] Any elevated OS write stays gated by `RequirePrivilege`
