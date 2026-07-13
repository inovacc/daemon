# Architecture
<!-- rev:002 -->

Current-state architecture of `github.com/inovacc/daemon` (a pure library; package `daemon`
at the module root — see ADR-0002). For the original design rationale and the state machine,
see [SERVICE_LIFECYCLE.md](SERVICE_LIFECYCLE.md); for Windows autostart, [AUTOSTART.md](AUTOSTART.md).

## System overview

```mermaid
flowchart TB
    subgraph consumer["Consumer app (own binary)"]
        root["cobra root cmd"]
        serve["Options.Serve(ctx, Ports)"]
    end
    subgraph daemon["daemon library"]
        attach["AttachCommands"]
        svc["svc install/uninstall/start/... (kardianos/service)"]
        auto["autostart enable/disable/status (Windows)"]
        mon["RunMonitor — supervisor loop + fork-loop guard"]
        wrk["RunWorker — runs Options.Serve"]
        info["internal/serverinfo — server.json (monitor PID)"]
    end
    root -->|AttachCommands o| attach
    attach --> svc & auto & mon & wrk
    mon -->|spawn __worker| wrk
    wrk --> serve
    mon -->|Write PID| info
    svc -->|elevated, RequirePrivilege| os["OS init system"]
    auto -->|elevated| reg["registry Run key / schtasks"]
```

## Detached start (spawn chain — 3 roles)

```mermaid
sequenceDiagram
    participant U as user: service start
    participant D as daemonize (Start)
    participant M as __monitor
    participant W as __worker
    participant S as serverinfo
    U->>D: Start(o)
    D->>D: env-guard check; IsRunning?
    alt already running
        D-->>U: ErrAlreadyRunning (pid)
    else
        D->>M: spawn detached (buildMonitorArgs — NO worker role, NO ports)
        M->>S: Write(monitor PID)
        M->>W: spawn __worker --port N --grpc-port N (buildWorkerArgs)
        W->>W: Options.Serve(ctx, Ports)
        D->>S: poll IsRunning up to healthWaitTimeout (5s)
        alt serverinfo observed
            D-->>U: started: pid=N
        else timeout
            D-->>U: pid + ErrHealthCheckTimeout (unconfirmed)
        end
    end
```

## Supervisor lifecycle

```mermaid
sequenceDiagram
    participant M as monitor loop
    participant G as restartGuard (4 restarts / 60s)
    participant W as __worker
    loop until ctx cancelled
        M->>W: spawn
        W-->>M: exit code
        alt clean exit (0)
            M->>M: stop
        else crash (1)
            M->>G: record restart
            alt 4 restarts < 60s
                G-->>M: loop detected → abort (ExitError)
            else
                M->>M: backoff, respawn
            end
        else ExitRestart (3)
            M->>M: respawn (no crash count, no backoff)
        else ExitUpgrade (4)
            M->>M: re-exec new binary
        end
    end
    M->>W: on stop — taskkill /T /F (Win) | SIGTERM group (unix)
```

## Source layout

| File(s) | Responsibility |
|---------|----------------|
| `cobra.go` | `AttachCommands`, `RunWorker`; wires `service`, `svc`, `autostart` + hidden `__monitor`/`__worker` |
| `options.go` | `Options`, `Ports`, `withDefaults` (derives `portsExplicit`) |
| `args.go` | `buildMonitorArgs` (no worker role/ports) / `buildWorkerArgs` (always ports) |
| `daemonize.go` | `Start`/`Stop`; detached spawn + health wait (`ErrHealthCheckTimeout`) |
| `monitor.go`, `restartguard.go` | supervisor loop + sliding-window fork-loop guard |
| `svc.go` | `svc` group over kardianos/service; `--autostart` combined trigger |
| `autostart.go`, `autostart_windows.go`, `autostart_unix.go` | Windows launch-at-logon (unix = unsupported stub) |
| `spawn_*.go`, `reexec_*.go`, `stop_*.go` | platform detach / re-exec / stop (build-tagged) |
| `exitstatus.go`, `privilege*.go` | exit-code protocol; `RequirePrivilege` → exit 5 |
| `internal/serverinfo/` | monitor-PID `server.json` (write/read/IsRunning + stale self-heal) |
