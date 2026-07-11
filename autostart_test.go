package daemon

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// fakeAutostart records the verbs invoked on it so tests can assert the command
// layer parsed flags and gated privilege correctly.
type fakeAutostart struct {
	enable  []autostartCall
	disable []autostartCall
	entries []autostartEntry
	err     error
}

type autostartCall struct {
	method   autostartMethod
	elevated bool
}

func (f *fakeAutostart) Enable(m autostartMethod, e bool) error {
	f.enable = append(f.enable, autostartCall{m, e})
	return f.err
}

func (f *fakeAutostart) Disable(m autostartMethod, e bool) error {
	f.disable = append(f.disable, autostartCall{m, e})
	return f.err
}

func (f *fakeAutostart) Status() ([]autostartEntry, error) { return f.entries, f.err }

// withFakeAutostart installs fake behind the newAutostartManager seam and an
// elevated-token result behind isElevatedFn, restoring both on cleanup.
func withFakeAutostart(t *testing.T, fake autostartManager, elevated bool) {
	t.Helper()

	origMgr, origElev := newAutostartManager, isElevatedFn

	t.Cleanup(func() { newAutostartManager, isElevatedFn = origMgr, origElev })

	newAutostartManager = func(Options, []string) (autostartManager, error) { return fake, nil }
	isElevatedFn = func() bool { return elevated }
}

// runAutostart executes the wired `autostart` subtree with args and returns the
// combined stdout/stderr plus the RunE error.
func runAutostart(t *testing.T, o Options, args ...string) (string, error) {
	t.Helper()

	root := &cobra.Command{Use: "daemon"}
	root.AddCommand(autostartCommand(o))

	var buf bytes.Buffer

	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)

	err := root.Execute()

	return buf.String(), err
}

func TestValidateServiceName(t *testing.T) {
	for _, tc := range []struct {
		name  string
		valid bool
	}{
		{"daemon", true},
		{"my-app_1.2", true},
		{"MyApp", true},
		{"", false},
		{"   ", false},
		{"has space", false},
		{"semi;colon", false},
		{`slash\path`, false},
		{`quote"x`, false},
		{"amp&y", false},
	} {
		err := validateServiceName(tc.name)
		if tc.valid && err != nil {
			t.Errorf("validateServiceName(%q) = %v, want nil", tc.name, err)
		}

		if !tc.valid && err == nil {
			t.Errorf("validateServiceName(%q) = nil, want error", tc.name)
		}
	}
}

func TestParseMethod(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want autostartMethod
		ok   bool
	}{
		{"startup", methodStartup, true},
		{"TaskScheduler", methodTaskScheduler, true},
		{"  startup  ", methodStartup, true},
		{"bogus", "", false},
	} {
		got, err := parseMethod(tc.in)
		if tc.ok && (err != nil || got != tc.want) {
			t.Errorf("parseMethod(%q) = %q, %v; want %q, nil", tc.in, got, err, tc.want)
		}

		if !tc.ok && err == nil {
			t.Errorf("parseMethod(%q) = nil error; want failure", tc.in)
		}
	}
}

func TestAutostartEnableCallsManager(t *testing.T) {
	fake := &fakeAutostart{}
	withFakeAutostart(t, fake, false)

	out, err := runAutostart(t, Options{ServiceName: "daemon"}, "autostart", "enable", "--method", "taskscheduler")
	if err != nil {
		t.Fatalf("enable returned %v", err)
	}

	if len(fake.enable) != 1 || fake.enable[0].method != methodTaskScheduler || fake.enable[0].elevated {
		t.Fatalf("Enable calls = %+v; want one taskscheduler/non-elevated", fake.enable)
	}

	if !strings.Contains(out, "enabled") {
		t.Fatalf("output %q missing 'enabled'", out)
	}
}

func TestAutostartEnableElevatedRequiresPrivilege(t *testing.T) {
	fake := &fakeAutostart{}
	withFakeAutostart(t, fake, false) // not elevated

	_, err := runAutostart(t, Options{ServiceName: "daemon"}, "autostart", "enable", "--elevated")
	if !errors.Is(err, ErrNeedsPrivilege) {
		t.Fatalf("err = %v; want ErrNeedsPrivilege", err)
	}

	if len(fake.enable) != 0 {
		t.Fatalf("Enable must not run without privilege; got %+v", fake.enable)
	}
}

func TestAutostartEnableElevatedProceedsWhenElevated(t *testing.T) {
	fake := &fakeAutostart{}
	withFakeAutostart(t, fake, true) // elevated

	if _, err := runAutostart(t, Options{ServiceName: "daemon"}, "autostart", "enable", "--elevated"); err != nil {
		t.Fatalf("enable --elevated returned %v", err)
	}

	if len(fake.enable) != 1 || !fake.enable[0].elevated {
		t.Fatalf("Enable calls = %+v; want one elevated", fake.enable)
	}
}

func TestAutostartDisableRejectsUnknownMethod(t *testing.T) {
	fake := &fakeAutostart{}
	withFakeAutostart(t, fake, false)

	_, err := runAutostart(t, Options{ServiceName: "daemon"}, "autostart", "disable", "--method", "bogus")
	if err == nil {
		t.Fatal("disable with unknown method: want error")
	}

	if len(fake.disable) != 0 {
		t.Fatalf("Disable must not run for a bad method; got %+v", fake.disable)
	}
}

func TestAutostartStatusReportsEnabledEntries(t *testing.T) {
	fake := &fakeAutostart{entries: []autostartEntry{
		{Method: methodStartup, Scope: "user", Enabled: true, Target: `"C:\d.exe" service`},
		{Method: methodTaskScheduler, Scope: "logon", Enabled: false},
	}}
	withFakeAutostart(t, fake, false)

	out, err := runAutostart(t, Options{ServiceName: "daemon"}, "autostart", "status")
	if err != nil {
		t.Fatalf("status returned %v", err)
	}

	if !strings.Contains(out, "startup (user)") {
		t.Fatalf("status output %q missing enabled startup entry", out)
	}

	if strings.Contains(out, "taskscheduler") {
		t.Fatalf("status output %q leaked disabled entry", out)
	}
}

func TestAutostartStatusEmpty(t *testing.T) {
	withFakeAutostart(t, &fakeAutostart{}, false)

	out, err := runAutostart(t, Options{ServiceName: "daemon"}, "autostart", "status")
	if err != nil {
		t.Fatalf("status returned %v", err)
	}

	if !strings.Contains(out, "not enabled") {
		t.Fatalf("status output %q missing 'not enabled'", out)
	}
}
