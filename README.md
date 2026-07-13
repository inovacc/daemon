# daemon
<!-- rev:001 -->

[![Go Reference](https://pkg.go.dev/badge/github.com/inovacc/daemon.svg)](https://pkg.go.dev/github.com/inovacc/daemon)
[![Go Version](https://img.shields.io/github/go-mod/go-version/inovacc/daemon)](go.mod)
[![License: BSD-3-Clause](https://img.shields.io/badge/License-BSD_3--Clause-blue.svg)](LICENSE)

A reusable Go **library** that gives any application the ability to run as a
long-lived background service — foreground supervisor (monitor → worker with a
fork-loop guard), OS-service registration, privilege handling, and Windows
launch-at-logon (Startup / Task Scheduler).

It is library-first: there is no binary in this module. You wire it into your own
Cobra root command and ship your own executable.

## Install

```bash
go get github.com/inovacc/daemon@latest
```

## Usage

```go
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/inovacc/daemon"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{Use: "myapp"}

	// serve is your long-running worker body; it must block until ctx is cancelled.
	serve := func(ctx context.Context, p daemon.Ports) error {
		slog.Info("serving", "http", p.HTTP, "grpc", p.GRPC)
		<-ctx.Done()
		return nil
	}

	if err := daemon.AttachCommands(root, daemon.Options{
		BinaryName: "myapp",
		Serve:      serve,
	}); err != nil {
		cobra.CheckErr(err)
	}

	if err := root.Execute(); err != nil {
		os.Exit(daemon.ExitCodeFor(err)) // maps privilege failures to exit 5
	}
}
```

`AttachCommands` wires these command groups onto your root:

| Command | What it does |
|---------|--------------|
| `service` / `service start` / `stop` / `status` | Foreground supervisor (monitor + worker) and detached background lifecycle |
| `svc install` / `uninstall` / `start` / `stop` / `restart` / `status` | Register with the OS init system (systemd / launchd / Windows service) — privileged |
| `svc install --autostart` | One step: install the elevated service **and** an elevated logon trigger that starts it |
| `autostart enable` / `disable` / `status` | Windows launch-at-logon via registry Run key or Task Scheduler (`--elevated` for all-users/SYSTEM) |

Privileged verbs are gated by an elevation check: without admin/root they print
re-run guidance and exit `5` (`ExitNeedsPrivilege`) without touching the system.

Windows autostart details (Startup vs Task Scheduler, the Google-style elevated
service + trigger shape) are in [docs/AUTOSTART.md](docs/AUTOSTART.md).

## Development

```bash
task test    # race + coverage
task check   # fmt, vet, lint, build, test
```

## License

BSD 3-Clause. See [LICENSE](LICENSE).
