# Autostart (Windows)

<!-- rev:002 -->

The `autostart` command registers the daemon to launch automatically, reflecting
how Google ships its own Windows software: a privileged always-on **service**
(`svc` group, LocalSystem) plus **logon/boot triggers**. The trigger mechanisms
mirror the two approaches Syncthing documents, driven from code instead of the
GUI.

For the full Google shape in one step, `svc install --autostart` installs the
elevated service **and** registers an elevated trigger that starts it (see
[Combined install](#combined-install-one-step) below).

## What Google does (the pattern being reflected)

On this machine Google registers, all as `LocalSystem` (elevated):

| Component | Mechanism | Reflected by |
|-----------|-----------|--------------|
| `GoogleChromeElevationService` | Windows service | `daemon svc install` |
| `GoogleUpdaterService…` | Windows service | `daemon svc install` |
| Google update tasks | Task Scheduler, elevated | `daemon autostart enable --method taskscheduler --elevated` |

## Methods

| Method | Mechanism | Scope (default) | `--elevated` |
|--------|-----------|-----------------|--------------|
| `startup` | Registry `Run` key | `HKCU` (current user) | `HKLM` (all users, needs admin) |
| `taskscheduler` | `schtasks` ONLOGON task | current user, limited | `SYSTEM` + `/RL HIGHEST` (needs admin) |

Both launch `"<exe>" service` — the monitor→worker supervisor.

## Usage

```powershell
daemon autostart enable                                   # HKCU Run key (per-user)
daemon autostart enable --method taskscheduler            # per-user logon task
daemon autostart enable --method taskscheduler --elevated # SYSTEM task, highest privileges (admin)
daemon autostart enable --elevated                        # HKLM Run key, all users (admin)
daemon autostart status                                   # list active registrations
daemon autostart disable [--method …] [--elevated]        # remove
```

`--elevated` is gated by the same `RequirePrivilege` guard as the `svc` verbs:
without admin it prints re-run guidance and exits `5` (`ExitNeedsPrivilege`)
without touching the system.

## Combined install (one step)

`svc install --autostart` mirrors Google exactly — an elevated service **plus** an
elevated logon trigger — in a single privileged command:

```powershell
daemon svc install --autostart                              # service + SYSTEM Task Scheduler trigger
daemon svc install --autostart --autostart-method startup   # service + HKLM Run key
daemon svc uninstall --autostart [--autostart-method …]     # remove both
```

Behavior:

- The trigger launches `"<exe>" svc start` (not the standalone `service`
  supervisor), so it only asks the SCM to start the already-installed service —
  one process, no duplicate supervisor.
- The trigger is always registered **elevated** (SYSTEM / HKLM), since
  `svc install` already runs elevated.
- Fail-fast ordering: the trigger manager is validated (platform + method) *before*
  the service is installed, so an unsupported platform or bad `--autostart-method`
  leaves nothing half-configured.
- On non-Windows, `--autostart` returns the "Windows-only; use `svc`" error before
  installing.

## Design notes

- **Registry** writes use `golang.org/x/sys/windows/registry` (no new dependency).
- **Task Scheduler** uses `schtasks.exe` (`/Create /SC ONLOGON`, `/RU SYSTEM /RL
  HIGHEST` when elevated), avoiding COM.
- Platform-split: `autostart_windows.go` implements it; `autostart_other.go`
  returns a friendly "Windows-only; use `svc` (systemd/launchd)" error.
- The manager sits behind the `newAutostartManager` seam for testing, matching
  the existing `newOSService` / `isElevatedFn` patterns.

## References

- Syncthing — Autostart on Windows (Startup):
  <https://docs.syncthing.net/users/autostart.html#autostart-windows-startup>
- Syncthing — Autostart using Windows Task Scheduler:
  <https://docs.syncthing.net/users/autostart.html#autostart-windows-taskschd>
- Microsoft — Task Scheduler:
  <https://learn.microsoft.com/windows/win32/TaskSchd/task-scheduler-start-page>
