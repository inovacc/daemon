package daemon

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func TestAttachRequiresServe(t *testing.T) {
	err := AttachCommands(&cobra.Command{Use: "root"}, Options{BinaryName: "t"})
	if err == nil {
		t.Fatal("AttachCommands must reject Options without a Serve func")
	}
}

func TestAttachRegistersServiceAndHiddenCommands(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	err := AttachCommands(root, Options{
		BinaryName: "t",
		Serve:      func(context.Context, Ports) error { return nil },
	})
	if err != nil {
		t.Fatalf("AttachCommands: %v", err)
	}

	want := map[string]bool{"service": false, "__monitor": true, "__worker": true}
	got := map[string]bool{}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			got[c.Name()] = c.Hidden
		}
	}
	for name, hidden := range want {
		h, ok := got[name]
		if !ok {
			t.Fatalf("command %q not registered", name)
		}
		if h != hidden {
			t.Fatalf("command %q hidden=%v, want %v", name, h, hidden)
		}
	}
}

func TestRunWorkerInvokesServeWithPorts(t *testing.T) {
	var gotPorts Ports
	sentinel := errors.New("from serve")
	err := RunWorker(context.Background(), Options{
		BinaryName: "t",
		HTTPPort:   7001,
		GRPCPort:   7002,
		Serve: func(_ context.Context, p Ports) error {
			gotPorts = p
			return sentinel
		},
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("RunWorker should return Serve's error, got %v", err)
	}
	if gotPorts.HTTP != 7001 || gotPorts.GRPC != 7002 {
		t.Fatalf("Serve got wrong ports: %+v", gotPorts)
	}
}

func TestAttachRegistersSvcGroup(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	if err := AttachCommands(root, Options{
		BinaryName: "t",
		Serve:      func(context.Context, Ports) error { return nil },
	}); err != nil {
		t.Fatalf("AttachCommands: %v", err)
	}

	var svc *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "svc" {
			svc = c
		}
	}
	if svc == nil {
		t.Fatal("svc group not registered by AttachCommands")
	}
	if svc.Hidden {
		t.Fatal("svc group must be visible")
	}

	want := map[string]bool{ // name -> hidden
		"install": false, "uninstall": false, "start": false,
		"stop": false, "restart": false, "status": false, "run": true,
	}
	got := map[string]bool{}
	for _, c := range svc.Commands() {
		got[c.Name()] = c.Hidden
	}
	for name, hidden := range want {
		h, ok := got[name]
		if !ok {
			t.Fatalf("svc subcommand %q not registered", name)
		}
		if h != hidden {
			t.Fatalf("svc %q hidden=%v, want %v", name, h, hidden)
		}
	}
}
