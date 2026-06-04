package daemon

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

func TestProgramStartIsNonBlockingAndStopCancels(t *testing.T) {
	started := make(chan struct{})
	releasedCtx := make(chan struct{})

	p := newProgram(Options{BinaryName: "t"}.withDefaults())
	// Replace the supervisor body with a controllable blocker.
	p.run = func(ctx context.Context, _ Options) error {
		close(started)
		<-ctx.Done() // unblocks only when Stop cancels
		close(releasedCtx)
		return ctx.Err()
	}

	// Start must NOT block: it launches the supervisor in a goroutine.
	if err := p.Start(nil); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not launch the supervisor goroutine")
	}

	// Stop must cancel the context and return well within the budget.
	stopReturned := make(chan error, 1)
	go func() { stopReturned <- p.Stop(nil) }()
	select {
	case err := <-stopReturned:
		if err != nil {
			t.Fatalf("Stop returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not return within budget")
	}
	select {
	case <-releasedCtx:
	case <-time.After(time.Second):
		t.Fatal("supervisor context was not cancelled by Stop")
	}
}

func TestProgramStopWaitsThenGivesUp(t *testing.T) {
	// A supervisor that ignores cancellation: Stop must still return (after the
	// time.After fallback fires), not hang forever.
	p := newProgram(Options{BinaryName: "t"}.withDefaults())
	p.stopTimeout = 50 * time.Millisecond // shrink the budget for the test
	p.run = func(ctx context.Context, _ Options) error {
		<-make(chan struct{}) // block forever, ignoring ctx
		return nil
	}
	_ = p.Start(nil)

	done := make(chan error, 1)
	go func() { done <- p.Stop(nil) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not honour its fallback timeout")
	}
}

// fakeOSService records which methods were called.
type fakeOSService struct {
	calls  []string
	status service.Status
	err    error
}

func (f *fakeOSService) Install() error   { f.calls = append(f.calls, "install"); return f.err }
func (f *fakeOSService) Uninstall() error { f.calls = append(f.calls, "uninstall"); return f.err }
func (f *fakeOSService) Start() error     { f.calls = append(f.calls, "start"); return f.err }
func (f *fakeOSService) Stop() error      { f.calls = append(f.calls, "stop"); return f.err }
func (f *fakeOSService) Restart() error   { f.calls = append(f.calls, "restart"); return f.err }
func (f *fakeOSService) Run() error       { f.calls = append(f.calls, "run"); return f.err }
func (f *fakeOSService) Status() (service.Status, error) {
	f.calls = append(f.calls, "status")
	return f.status, f.err
}

// findSub locates a direct subcommand by name.
func findSub(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	t.Fatalf("subcommand %q not found under %q", name, parent.Name())
	return nil
}

func withFakeOSService(t *testing.T, fake *fakeOSService) {
	t.Helper()
	prev := newOSService
	newOSService = func(Options) (osService, error) { return fake, nil }
	t.Cleanup(func() { newOSService = prev })
}

// withElevated forces the privilege seam to report elevated for the duration of the
// test, so RunEs of the mutating svc verbs (now gated by RequirePrivilege) reach the
// osService seam on this non-elevated host.
func withElevated(t *testing.T) {
	t.Helper()
	prev := isElevatedFn
	isElevatedFn = func() bool { return true }
	t.Cleanup(func() { isElevatedFn = prev })
}

func TestSvcVerbsDispatchToOSService(t *testing.T) {
	cases := []struct{ verb, want string }{
		{"install", "install"},
		{"uninstall", "uninstall"},
		{"start", "start"},
		{"stop", "stop"},
		{"restart", "restart"},
		{"status", "status"},
		{"run", "run"},
	}
	for _, tc := range cases {
		t.Run(tc.verb, func(t *testing.T) {
			fake := &fakeOSService{status: service.StatusRunning}
			withFakeOSService(t, fake)
			withElevated(t)

			grp := svcCommand(Options{BinaryName: "t"}.withDefaults())
			sub := findSub(t, grp, tc.verb)
			sub.SetContext(context.Background())
			var out bytes.Buffer
			sub.SetOut(&out)
			sub.SetErr(&out)
			if err := sub.RunE(sub, nil); err != nil {
				t.Fatalf("verb %q RunE: %v", tc.verb, err)
			}
			if len(fake.calls) != 1 || fake.calls[0] != tc.want {
				t.Fatalf("verb %q called %v, want [%s]", tc.verb, fake.calls, tc.want)
			}
		})
	}
}

func TestSvcRunIsHidden(t *testing.T) {
	grp := svcCommand(Options{BinaryName: "t"}.withDefaults())
	run := findSub(t, grp, "run")
	if !run.Hidden {
		t.Fatal("svc run must be Hidden")
	}
}

func TestSvcVerbPropagatesError(t *testing.T) {
	sentinel := errors.New("boom")
	fake := &fakeOSService{err: sentinel}
	withFakeOSService(t, fake)
	withElevated(t)

	grp := svcCommand(Options{BinaryName: "t"}.withDefaults())
	install := findSub(t, grp, "install")
	install.SetContext(context.Background())
	var out bytes.Buffer
	install.SetOut(&out)
	install.SetErr(&out)
	if err := install.RunE(install, nil); !errors.Is(err, sentinel) {
		t.Fatalf("install RunE error = %v, want wraps %v", err, sentinel)
	}
}

// TestRealOSServiceEmptyServiceName asserts realOSService returns the friendly
// wrapped error (the guard before service.New) when called WITHOUT withDefaults,
// so BinaryName/ServiceName are empty.
func TestRealOSServiceEmptyServiceName(t *testing.T) {
	s, err := realOSService(Options{})
	if err == nil {
		t.Fatal("realOSService(Options{}) must return an error for empty ServiceName")
	}
	if s != nil {
		t.Fatalf("realOSService must return a nil osService on error, got %v", s)
	}
	if !strings.Contains(err.Error(), "ServiceName is empty") {
		t.Fatalf("error %q must mention that ServiceName is empty", err.Error())
	}
}
