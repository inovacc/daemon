package daemon

import (
	"testing"
	"time"
)

func TestDefaultsFillZeroValues(t *testing.T) {
	o := Options{BinaryName: "myapp"}.withDefaults()

	if o.HTTPPort != DefaultHTTPPort || o.GRPCPort != DefaultGRPCPort {
		t.Fatalf("default ports not applied: http=%d grpc=%d", o.HTTPPort, o.GRPCPort)
	}
	if o.GuardSize != defaultGuardSize || o.GuardWindow != defaultGuardWindow {
		t.Fatalf("guard defaults not applied: size=%d window=%v", o.GuardSize, o.GuardWindow)
	}
	if o.MonitorCmd != "__monitor" || o.WorkerCmd != "__worker" {
		t.Fatalf("hidden command names not defaulted: %q %q", o.MonitorCmd, o.WorkerCmd)
	}
	if o.ServiceName == "" || o.DataDir == "" {
		t.Fatalf("ServiceName/DataDir should be derived: %q %q", o.ServiceName, o.DataDir)
	}
}

func TestDefaultsDoNotOverrideExplicitValues(t *testing.T) {
	o := Options{
		BinaryName:  "myapp",
		HTTPPort:    1111,
		GRPCPort:    2222,
		GuardSize:   9,
		GuardWindow: 5 * time.Second,
		ServiceName: "Custom",
		DataDir:     "/tmp/x",
	}.withDefaults()

	if o.HTTPPort != 1111 || o.GRPCPort != 2222 || o.GuardSize != 9 ||
		o.GuardWindow != 5*time.Second || o.ServiceName != "Custom" || o.DataDir != "/tmp/x" {
		t.Fatalf("explicit values were overridden: %+v", o)
	}
}
