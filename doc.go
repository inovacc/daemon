// Package daemon is a reusable service/daemon layer consolidating the supervisor
// pattern (monitor + inner worker process, restart loop, crash recovery, platform
// service install) and the gRPC thin-client daemon pattern (health, idle
// auto-shutdown, discovery + auto-start) used across the owner's projects.
//
// It is library-first: consumers wire it into their own Cobra root command. The
// authoritative design and lifecycle state machine live in docs/SERVICE_LIFECYCLE.md.
//
// Intended public surface (see ROADMAP.md for status):
//
//	daemon.AttachCommands(rootCmd, daemon.Options{...}) // wires `service` + hidden `__monitor`/`__worker`
//	daemon.RunMonitor(opts)                             // supervisor restart loop (fork-loop guarded)
//	daemon.RunWorker(opts)                              // inner process: bind -> serverinfo -> initServices -> serve
//
// Hard invariants enforced by the module so consumers cannot get them wrong:
//   - daemonize spawns __monitor (never __worker); serverinfo stores the MONITOR pid.
//   - __worker skips the singleton IsRunning() check; heavy init is deferred past port bind.
//   - the monitor restart loop is fork/spawn loop-hell guarded (sliding window + backoff;
//     ExitRestart bypasses the crash counter).
package daemon
