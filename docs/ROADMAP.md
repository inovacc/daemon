# Roadmap

## Current Status
**Overall Progress:** ~35% - Foreground supervisor + public API implemented & unit-tested (19 tests)

## Phases

### Phase 1: Foundation [DONE]
- [x] Project scaffold (structure, tooling, CI, BSD-3 license)
- [x] Service-layer design spec (`docs/SERVICE_LIFECYCLE.md`)
- [x] Public library API surface at the module root (Options, AttachCommands, RunMonitor/RunWorker)
- [x] `serverinfo` package (monitor-PID file: write/read/IsRunning + stale self-heal), platform liveness

### Phase 2: Core Features [IN PROGRESS]
- [x] Monitor restart loop + sliding-window fork-loop guard + exponential backoff (unit-tested)
- [x] `__monitor` / `__worker` hidden Cobra commands + arg-builders (monitor never carries worker role)
- [ ] Detached `service start` (daemonize → spawn `__monitor`) + env-guard self-spawn protection
- [ ] Platform detach + stop (taskkill /T /F | SIGINT) build-tagged files; window hiding
- [ ] gRPC daemon path (server + IdleTracker + discovery) lifted from kody
- [ ] kardianos/service install/uninstall integration
- [ ] Integration tests: real worker spawn, crash→restart, TestMain hard timeout

### Phase 3: Polish & Release [NOT STARTED]
- [ ] Port weaver and kody onto the module (behind deprecation dates)
- [ ] Stress/zombie tests + TestMain hard timeouts
- [ ] v1.0.0 release
