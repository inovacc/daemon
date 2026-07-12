package daemon

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestValidateRejectsBadPortsAndNames(t *testing.T) {
	serve := func(context.Context, Ports) error { return nil }

	cases := []struct {
		name string
		opts Options
		want string // substring expected in the error, "" == must succeed
	}{
		{"negative http", Options{BinaryName: "t", HTTPPort: -1, Serve: serve}, "HTTPPort"},
		{"http too high", Options{BinaryName: "t", HTTPPort: 70000, Serve: serve}, "HTTPPort"},
		{"grpc too high", Options{BinaryName: "t", GRPCPort: 99999, Serve: serve}, "GRPCPort"},
		{"bad service name", Options{BinaryName: "t", ServiceName: "bad name!", Serve: serve}, "invalid character"},
		{"bad binary name", Options{BinaryName: "my app", Serve: serve}, "invalid character"},
		{"valid explicit ports", Options{BinaryName: "t", HTTPPort: 8080, GRPCPort: 8081, Serve: serve}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := AttachCommands(&cobra.Command{Use: "root"}, tc.opts)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("AttachCommands should accept %+v, got %v", tc.opts, err)
				}

				return
			}

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("AttachCommands error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

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
