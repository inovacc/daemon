package daemon

import (
	"slices"
	"testing"
)

func TestWorkerArgsAlwaysCarryRoleAndPorts(t *testing.T) {
	o := Options{BinaryName: "myapp"}.withDefaults()
	args := o.buildWorkerArgs()

	if len(args) == 0 || args[0] != "__worker" {
		t.Fatalf("worker args must start with the hidden worker command: %v", args)
	}
	// Ports are ALWAYS present (visible in `ps`), defaulting to 9500/9501.
	if !hasFlagValue(args, "--port", "9500") || !hasFlagValue(args, "--grpc-port", "9501") {
		t.Fatalf("worker args must always include --port and --grpc-port: %v", args)
	}
}

func TestMonitorArgsNeverCarryWorkerRoleOrPorts(t *testing.T) {
	o := Options{BinaryName: "myapp"}.withDefaults()
	args := o.buildMonitorArgs()

	if len(args) == 0 || args[0] != "__monitor" {
		t.Fatalf("monitor args must start with the hidden monitor command: %v", args)
	}
	// The classic bug: daemonize() spawning the worker. Monitor must NOT carry __worker,
	// and must NOT carry ports unless the user explicitly overrode them.
	if slices.Contains(args, "__worker") {
		t.Fatalf("monitor args must never contain the worker role: %v", args)
	}

	if slices.Contains(args, "--port") || slices.Contains(args, "--grpc-port") {
		t.Fatalf("monitor args must not carry ports when not user-overridden: %v", args)
	}
}

func TestMonitorArgsCarryUserOverriddenPorts(t *testing.T) {
	o := Options{BinaryName: "myapp", HTTPPort: 8080, GRPCPort: 8081, portsExplicit: true}.withDefaults()

	args := o.buildMonitorArgs()
	if !hasFlagValue(args, "--port", "8080") || !hasFlagValue(args, "--grpc-port", "8081") {
		t.Fatalf("monitor args should forward user-overridden ports: %v", args)
	}
}

func TestMonitorArgsForwardConsumerSetPorts(t *testing.T) {
	// A consumer can only set the PUBLIC HTTPPort/GRPCPort fields (portsExplicit is
	// unexported). withDefaults must derive that non-default ports are explicit so
	// the monitor forwards them — otherwise the worker silently reverts to 9500/9501.
	o := Options{BinaryName: "myapp", HTTPPort: 8080, GRPCPort: 8081}.withDefaults()

	if !o.portsExplicit {
		t.Fatalf("withDefaults should derive portsExplicit from consumer-set ports")
	}

	args := o.buildMonitorArgs()
	if !hasFlagValue(args, "--port", "8080") || !hasFlagValue(args, "--grpc-port", "8081") {
		t.Fatalf("monitor args should forward consumer-set ports: %v", args)
	}
}

func TestMonitorArgsForwardWhenSinglePortOverridden(t *testing.T) {
	// Overriding only one port still marks the pair explicit; the other keeps its default.
	o := Options{BinaryName: "myapp", HTTPPort: 8080}.withDefaults()
	if !o.portsExplicit {
		t.Fatalf("overriding one port should mark ports explicit")
	}

	args := o.buildMonitorArgs()
	if !hasFlagValue(args, "--port", "8080") || !hasFlagValue(args, "--grpc-port", "9501") {
		t.Fatalf("monitor args should forward overridden port + defaulted grpc: %v", args)
	}
}

func TestMonitorArgsIdempotentDefaultsStayImplicit(t *testing.T) {
	// withDefaults is applied more than once in the real flow (AttachCommands then
	// Start). A second pass over already-defaulted ports must NOT flip portsExplicit,
	// or every consumer would spuriously forward the compiled-in defaults.
	o := Options{BinaryName: "myapp"}.withDefaults().withDefaults()
	if o.portsExplicit {
		t.Fatalf("double withDefaults must not mark default ports explicit")
	}

	if args := o.buildMonitorArgs(); slices.Contains(args, "--port") {
		t.Fatalf("monitor args must not carry default ports after double withDefaults: %v", args)
	}
}

func hasFlagValue(args []string, flag, val string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == val {
			return true
		}
	}

	return false
}
