package daemon

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

// findSvcVerb locates a leaf verb under the `svc` parent built by AttachCommands.
func findSvcVerb(t *testing.T, root *cobra.Command, name string) *cobra.Command {
	t.Helper()

	for _, svc := range root.Commands() {
		if svc.Name() != "svc" {
			continue
		}

		for _, v := range svc.Commands() {
			if v.Name() == name {
				return v
			}
		}
	}

	t.Fatalf("verb %q not found under svc", name)

	return nil
}

func newTestRoot(t *testing.T) *cobra.Command {
	t.Helper()

	root := &cobra.Command{Use: "daemon"}
	if err := AttachCommands(root, Options{
		BinaryName: "daemon",
		DataDir:    t.TempDir(),
		Serve:      func(context.Context, Ports) error { return nil },
	}); err != nil {
		t.Fatalf("AttachCommands: %v", err)
	}

	return root
}

var privilegedSvcVerbs = []string{"install", "uninstall", "start", "stop", "restart"}

func TestSvcPrivilegedVerbsBlockedWhenNotElevated(t *testing.T) {
	orig := isElevatedFn

	t.Cleanup(func() { isElevatedFn = orig })

	isElevatedFn = func() bool { return false }

	root := newTestRoot(t)
	for _, name := range privilegedSvcVerbs {
		t.Run(name, func(t *testing.T) {
			v := findSvcVerb(t, root, name)
			v.SetContext(context.Background())

			var stderr bytes.Buffer
			v.SetErr(&stderr)

			err := v.RunE(v, nil)
			if !errors.Is(err, ErrNeedsPrivilege) {
				t.Fatalf("svc %s RunE = %v, want ErrNeedsPrivilege", name, err)
			}

			if stderr.Len() == 0 {
				t.Fatalf("svc %s printed no guidance", name)
			}
		})
	}
}

func TestSvcPrivilegedVerbsProceedWhenElevated(t *testing.T) {
	origElev := isElevatedFn
	origNew := newOSService

	t.Cleanup(func() {
		isElevatedFn = origElev
		newOSService = origNew
	})

	isElevatedFn = func() bool { return true }

	fake := &fakeOSService{status: service.StatusRunning}
	newOSService = func(Options) (osService, error) { return fake, nil }

	root := newTestRoot(t)
	want := map[string]string{
		"install":   "install",
		"uninstall": "uninstall",
		"start":     "start",
		"stop":      "stop",
		"restart":   "restart",
	}

	for _, name := range privilegedSvcVerbs {
		t.Run(name, func(t *testing.T) {
			v := findSvcVerb(t, root, name)
			v.SetContext(context.Background())

			var out bytes.Buffer
			v.SetOut(&out)
			v.SetErr(&out)

			if err := v.RunE(v, nil); errors.Is(err, ErrNeedsPrivilege) {
				t.Fatalf("svc %s returned ErrNeedsPrivilege while elevated", name)
			}

			found := false

			for _, c := range fake.calls {
				if c == want[name] {
					found = true
				}
			}

			if !found {
				t.Fatalf("svc %s did not reach osService.%s when elevated; calls=%v", name, want[name], fake.calls)
			}
		})
	}
}

func TestSvcStatusAndRunNotGated(t *testing.T) {
	origElev := isElevatedFn
	origNew := newOSService

	t.Cleanup(func() {
		isElevatedFn = origElev
		newOSService = origNew
	})
	// Not elevated — status/run must STILL work (they are intentionally ungated).
	isElevatedFn = func() bool { return false }
	fake := &fakeOSService{status: service.StatusRunning}
	newOSService = func(Options) (osService, error) { return fake, nil }

	root := newTestRoot(t)
	for _, name := range []string{"status", "run"} {
		t.Run(name, func(t *testing.T) {
			v := findSvcVerb(t, root, name)
			v.SetContext(context.Background())

			var out bytes.Buffer
			v.SetOut(&out)
			v.SetErr(&out)

			if err := v.RunE(v, nil); errors.Is(err, ErrNeedsPrivilege) {
				t.Fatalf("svc %s was gated but must be unprivileged", name)
			}
		})
	}
}

// TestSvcInstallEndToEndSurfacesPrivilegeError drives the full cobra dispatch
// (root.Execute with args) and asserts ErrNeedsPrivilege surfaces to the caller
// when the process is not elevated.
func TestSvcInstallEndToEndSurfacesPrivilegeError(t *testing.T) {
	orig := isElevatedFn

	t.Cleanup(func() { isElevatedFn = orig })

	isElevatedFn = func() bool { return false }

	root := newTestRoot(t)
	root.SilenceUsage = true
	root.SilenceErrors = true

	var stderr bytes.Buffer
	root.SetOut(&stderr)
	root.SetErr(&stderr)
	root.SetArgs([]string{"svc", "install"})

	err := root.Execute()
	if !errors.Is(err, ErrNeedsPrivilege) {
		t.Fatalf("root.Execute() = %v, want ErrNeedsPrivilege", err)
	}

	if stderr.Len() == 0 {
		t.Fatalf("expected guidance on stderr, got none")
	}
}
