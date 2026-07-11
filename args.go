package daemon

import "strconv"

// buildWorkerArgs constructs the args the monitor uses to spawn the inner worker.
// The worker role and BOTH ports are ALWAYS present so they show up in process
// listings (ps / Task Manager), defaulting to 9500/9501 when not overridden.
func (o Options) buildWorkerArgs() []string {
	return []string{
		o.WorkerCmd,
		"--port", strconv.Itoa(o.HTTPPort),
		"--grpc-port", strconv.Itoa(o.GRPCPort),
	}
}

// buildMonitorArgs constructs the args used to spawn the detached daemon as a MONITOR.
//
// Critical: it must NEVER contain the worker role or `--inner` — if daemonize spawns
// the worker directly, monitorMain is skipped, server.json is never written, and
// stop/status silently break. Ports are forwarded only when the user explicitly
// overrode them (otherwise the worker's defaults apply).
func (o Options) buildMonitorArgs() []string {
	args := []string{o.MonitorCmd}
	if o.portsExplicit {
		args = append(args, "--port", strconv.Itoa(o.HTTPPort), "--grpc-port", strconv.Itoa(o.GRPCPort))
	}

	return args
}
