# Bug Tracker

## Open Bugs

_No known open bugs._

## Fixed Bugs

| Bug | Severity | Fix | Date |
|-----|----------|-----|------|
| Failed upgrade re-exec busy-looped (`attempt=0; continue`, no backoff) → unbounded CPU spin | High | Route the failure through the crash guard + backoff | 2026-07-12 (`c1336aa`) |
| Consumer-set ports (and `ServiceName` on the OS-service path) reached the OS unvalidated | Medium | Validate ports + name in `AttachCommands` / `realOSService` | 2026-07-12 (`faff04f`) |
| `Stop()` silently discarded the `serverinfo.Remove()` error | Low | Log it as a non-fatal warning | 2026-07-11 (`406ed72`) |
| A corrupt `server.json` was left in place, not self-healed | Low | `IsRunning` removes an unreadable pid file | 2026-07-12 (`7bb8627`) |
| `childEnvName` mapped e.g. `my-app`/`my_app` to the same recursion-guard var | Low | Append an FNV hash of the original name | 2026-07-12 (`7bb8627`) |
