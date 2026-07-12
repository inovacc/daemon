// Package daemon is a reusable service/daemon layer: it gives a consumer application
// the ability to run as a long-lived background service. It consolidates the supervisor
// pattern (a monitor process supervising an inner worker, with a restart loop and a
// fork-loop guard) with OS-service registration and Windows launch-at-logon, so a
// consumer wires it in with a single call and ships its own executable.
//
// It is library-first: consumers attach it to their own Cobra root command. The
// authoritative design and lifecycle state machine live in docs/SERVICE_LIFECYCLE.md.
//
// Public surface:
//
//	daemon.AttachCommands(root, daemon.Options{...}) // wires `service`, `svc`, `autostart` + hidden `__monitor`/`__worker`
//	daemon.RunMonitor(ctx, opts)                     // supervisor restart loop (fork-loop guarded)
//	daemon.RunWorker(ctx, opts)                      // inner process: runs Options.Serve under a signal-cancelled context
//	daemon.Start(opts) / daemon.Stop(opts)           // detached background lifecycle
//	daemon.ExitCodeFor(err)                          // maps ErrNeedsPrivilege -> exit 5
//
// Hard invariants enforced by the module so consumers cannot get them wrong:
//   - daemonize spawns __monitor (never __worker); serverinfo stores the MONITOR pid.
//   - the monitor never carries the worker role or ports; the worker always carries both.
//   - the monitor restart loop is fork/spawn loop-hell guarded (sliding window + backoff;
//     ExitRestart bypasses the crash counter, and a failed upgrade re-exec backs off like a crash).
//
// An optional gRPC daemon path (health, idle auto-shutdown, discovery) is planned behind
// an opt-in Option; see ROADMAP.md.
package daemon
