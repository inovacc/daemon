//go:build windows

package daemon

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// regOp records a registry Run-key mutation for assertion.
type regOp struct {
	root  registry.Key
	name  string
	value string
}

// fakeRunKeys is an in-memory runKeyStore for tests.
type fakeRunKeys struct {
	sets, dels []regOp
	vals       map[string]string // "root|name" -> value
	getErr     error             // when set, get returns it (simulates an unexpected registry failure)
}

func rkKey(root registry.Key, name string) string { return fmt.Sprintf("%d|%s", root, name) }

func (f *fakeRunKeys) set(root registry.Key, name, value string) error {
	f.sets = append(f.sets, regOp{root, name, value})

	if f.vals == nil {
		f.vals = map[string]string{}
	}

	f.vals[rkKey(root, name)] = value

	return nil
}

func (f *fakeRunKeys) del(root registry.Key, name string) error {
	f.dels = append(f.dels, regOp{root, name, ""})
	delete(f.vals, rkKey(root, name))

	return nil
}

func (f *fakeRunKeys) get(root registry.Key, name string) (string, bool, error) {
	if f.getErr != nil {
		return "", false, f.getErr
	}

	v, ok := f.vals[rkKey(root, name)]

	return v, ok, nil
}

// withWindowsAutostartSeams swaps the registry/schtasks seams for the test and a
// captor for schtasks args, restoring originals on cleanup. Returns a pointer to
// the captured schtasks arg-batches.
func withWindowsAutostartSeams(t *testing.T, rk runKeyStore, taskPresent bool) *[][]string {
	t.Helper()

	origKeys, origRun, origQuery := runKeys, runSchtasksFn, queryTaskFn

	t.Cleanup(func() { runKeys, runSchtasksFn, queryTaskFn = origKeys, origRun, origQuery })

	var calls [][]string

	if rk != nil {
		runKeys = rk
	}

	runSchtasksFn = func(args []string) error {
		calls = append(calls, args)
		return nil
	}
	queryTaskFn = func(string) (bool, error) { return taskPresent, nil }

	return &calls
}

func newWinAutostart() *windowsAutostart {
	return &windowsAutostart{name: "daemon", target: `"C:\app\daemon.exe" svc start`}
}

func TestWindowsRunKeyRoot(t *testing.T) {
	if runKeyRoot(true) != registry.LOCAL_MACHINE {
		t.Errorf("elevated hive = %v, want LOCAL_MACHINE", runKeyRoot(true))
	}

	if runKeyRoot(false) != registry.CURRENT_USER {
		t.Errorf("non-elevated hive = %v, want CURRENT_USER", runKeyRoot(false))
	}
}

func TestWindowsEnableStartupSelectsHive(t *testing.T) {
	for _, tc := range []struct {
		elevated bool
		want     registry.Key
	}{
		{false, registry.CURRENT_USER},
		{true, registry.LOCAL_MACHINE},
	} {
		fake := &fakeRunKeys{}
		withWindowsAutostartSeams(t, fake, false)

		w := newWinAutostart()
		if err := w.Enable(methodStartup, tc.elevated); err != nil {
			t.Fatalf("Enable(startup, %v): %v", tc.elevated, err)
		}

		if len(fake.sets) != 1 {
			t.Fatalf("elevated=%v: got %d set calls, want 1", tc.elevated, len(fake.sets))
		}

		got := fake.sets[0]
		if got.root != tc.want || got.name != "daemon" || got.value != w.target {
			t.Fatalf("elevated=%v: set = %+v, want root=%v name=daemon value=%q", tc.elevated, got, tc.want, w.target)
		}
	}
}

func TestWindowsEnableTaskArgs(t *testing.T) {
	for _, tc := range []struct {
		elevated bool
		wantElev bool
	}{
		{false, false},
		{true, true},
	} {
		calls := withWindowsAutostartSeams(t, nil, false)

		w := newWinAutostart()
		if err := w.Enable(methodTaskScheduler, tc.elevated); err != nil {
			t.Fatalf("Enable(task, %v): %v", tc.elevated, err)
		}

		if len(*calls) != 1 {
			t.Fatalf("elevated=%v: got %d schtasks calls, want 1", tc.elevated, len(*calls))
		}

		args := strings.Join((*calls)[0], " ")

		// Base ONLOGON registration is always present.
		for _, want := range []string{"/Create", "/TN daemon", "/TR", "/SC ONLOGON", "/F"} {
			if !strings.Contains(args, want) {
				t.Fatalf("elevated=%v: args %q missing %q", tc.elevated, args, want)
			}
		}

		hasElev := strings.Contains(args, "/RU SYSTEM") && strings.Contains(args, "/RL HIGHEST")
		if hasElev != tc.wantElev {
			t.Fatalf("elevated=%v: SYSTEM/HIGHEST present=%v, want %v (args=%q)", tc.elevated, hasElev, tc.wantElev, args)
		}
	}
}

func TestWindowsDisable(t *testing.T) {
	// startup -> registry delete on the selected hive.
	fake := &fakeRunKeys{}
	withWindowsAutostartSeams(t, fake, false)

	w := newWinAutostart()
	if err := w.Disable(methodStartup, true); err != nil {
		t.Fatalf("Disable(startup, elevated): %v", err)
	}

	if len(fake.dels) != 1 || fake.dels[0].root != registry.LOCAL_MACHINE || fake.dels[0].name != "daemon" {
		t.Fatalf("del calls = %+v, want one LOCAL_MACHINE/daemon", fake.dels)
	}

	// taskscheduler -> schtasks /Delete.
	calls := withWindowsAutostartSeams(t, nil, false)
	if err := w.Disable(methodTaskScheduler, false); err != nil {
		t.Fatalf("Disable(task): %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("got %d schtasks calls, want 1", len(*calls))
	}

	if args := strings.Join((*calls)[0], " "); !strings.Contains(args, "/Delete") || !strings.Contains(args, "/TN daemon") || !strings.Contains(args, "/F") {
		t.Fatalf("delete args = %q, want /Delete /TN daemon /F", args)
	}
}

func TestWindowsEnableDisableUnknownMethod(t *testing.T) {
	withWindowsAutostartSeams(t, &fakeRunKeys{}, false)

	w := newWinAutostart()
	if err := w.Enable("bogus", false); err == nil {
		t.Fatal("Enable(bogus): want error")
	}

	if err := w.Disable("bogus", false); err == nil {
		t.Fatal("Disable(bogus): want error")
	}
}

func TestWindowsStatus(t *testing.T) {
	// Per-user Run key present; machine absent; task present.
	fake := &fakeRunKeys{}
	w := newWinAutostart()
	_ = fake.set(registry.CURRENT_USER, w.name, w.target)

	withWindowsAutostartSeams(t, fake, true)

	entries, err := w.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	// entries[0] = user startup (enabled), [1] = machine startup (disabled), [2] = task (enabled).
	if !entries[0].Enabled || entries[0].Scope != "user" || entries[0].Target != w.target {
		t.Fatalf("user entry = %+v, want enabled/user/target", entries[0])
	}

	if entries[1].Enabled || entries[1].Scope != "machine" {
		t.Fatalf("machine entry = %+v, want disabled/machine", entries[1])
	}

	if !entries[2].Enabled || entries[2].Method != methodTaskScheduler {
		t.Fatalf("task entry = %+v, want enabled/taskscheduler", entries[2])
	}
}

func TestWindowsStatusAllAbsent(t *testing.T) {
	withWindowsAutostartSeams(t, &fakeRunKeys{}, false) // empty registry, no task

	w := newWinAutostart()

	entries, err := w.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	for i, e := range entries {
		if e.Enabled {
			t.Fatalf("entry[%d] = %+v, want all disabled", i, e)
		}
	}
}

func TestWindowsStatusSurfacesRunKeyError(t *testing.T) {
	// An unexpected registry error (e.g. permission denied) must be surfaced by
	// Status, not silently reported as "not enabled".
	wantErr := errors.New("access denied")
	withWindowsAutostartSeams(t, &fakeRunKeys{getErr: wantErr}, false)

	w := newWinAutostart()
	if _, err := w.Status(); !errors.Is(err, wantErr) {
		t.Fatalf("Status err = %v, want %v", err, wantErr)
	}
}

func TestWindowsStatusSurfacesTaskError(t *testing.T) {
	// The registry side is clean; the task query fails unexpectedly. Status surfaces it.
	origKeys, origRun, origQuery := runKeys, runSchtasksFn, queryTaskFn

	t.Cleanup(func() { runKeys, runSchtasksFn, queryTaskFn = origKeys, origRun, origQuery })

	wantErr := errors.New("schtasks.exe missing")
	runKeys = &fakeRunKeys{}
	queryTaskFn = func(string) (bool, error) { return false, wantErr }

	w := newWinAutostart()
	if _, err := w.Status(); !errors.Is(err, wantErr) {
		t.Fatalf("Status err = %v, want %v", err, wantErr)
	}
}

func TestRealAutostartManagerBuildsTarget(t *testing.T) {
	mgr, err := realAutostartManager(Options{ServiceName: "daemon"}, []string{"svc", "start"})
	if err != nil {
		t.Fatalf("realAutostartManager: %v", err)
	}

	w, ok := mgr.(*windowsAutostart)
	if !ok {
		t.Fatalf("got %T, want *windowsAutostart", mgr)
	}

	if w.name != "daemon" {
		t.Errorf("name = %q, want daemon", w.name)
	}

	if !strings.HasSuffix(w.target, `" svc start`) {
		t.Errorf("target = %q, want it to end with quoted exe + ' svc start'", w.target)
	}
}
