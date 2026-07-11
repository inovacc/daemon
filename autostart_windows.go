//go:build windows

package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// runKeyPath is the classic per-logon autostart location under both HKCU (current
// user) and HKLM (all users). Reflects Syncthing's "Startup" method.
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// runKeyStore abstracts the registry Run-key operations so the Startup-method
// logic (hive selection, status assembly) is testable without touching the real
// registry. del treats an already-absent value as success; get reports presence.
type runKeyStore interface {
	set(root registry.Key, name, value string) error
	del(root registry.Key, name string) error
	// get reports the value's presence. A missing key/value is (\"\", false, nil);
	// err is non-nil only for an UNEXPECTED failure (e.g. permission denied), which
	// Status surfaces instead of silently reporting \"not enabled\".
	get(root registry.Key, name string) (value string, present bool, err error)
}

// Seams overridden in tests; production points at the real OS-backed impls below.
var (
	runKeys      runKeyStore = winRunKeyStore{}
	runSchtasksFn             = runSchtasks
	queryTaskFn              = queryTask
)

// windowsAutostart implements autostartManager against the registry Run key
// (Startup method) and schtasks.exe (Task Scheduler method).
type windowsAutostart struct {
	name   string // registry value name / scheduled-task name (Options.ServiceName)
	target string // full command line the entry launches: "<exe>" service
}

// realAutostartManager builds the Windows autostart manager, resolving the target
// command line once from the running executable and the launch argv.
func realAutostartManager(o Options, launch []string) (autostartManager, error) {
	if err := validateServiceName(o.ServiceName); err != nil {
		return nil, err
	}

	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve executable: %w", err)
	}

	target := fmt.Sprintf("%q", exe)
	if len(launch) > 0 {
		target += " " + strings.Join(launch, " ")
	}

	return &windowsAutostart{
		name:   o.ServiceName,
		target: target,
	}, nil
}

func (w *windowsAutostart) Enable(method autostartMethod, elevated bool) error {
	switch method {
	case methodStartup:
		return w.writeRunKey(elevated)
	case methodTaskScheduler:
		return w.createTask(elevated)
	default:
		return fmt.Errorf("daemon: unknown autostart method %q", method)
	}
}

func (w *windowsAutostart) Disable(method autostartMethod, elevated bool) error {
	switch method {
	case methodStartup:
		return w.deleteRunKey(elevated)
	case methodTaskScheduler:
		return w.deleteTask()
	default:
		return fmt.Errorf("daemon: unknown autostart method %q", method)
	}
}

func (w *windowsAutostart) Status() ([]autostartEntry, error) {
	userKey, err := w.runKeyStatus(false)
	if err != nil {
		return nil, err
	}

	machineKey, err := w.runKeyStatus(true)
	if err != nil {
		return nil, err
	}

	task, err := w.taskStatus()
	if err != nil {
		return nil, err
	}

	return []autostartEntry{userKey, machineKey, task}, nil
}

// runKeyRoot picks the hive: HKLM for the elevated all-users registration, HKCU
// for the per-user one.
func runKeyRoot(elevated bool) registry.Key {
	if elevated {
		return registry.LOCAL_MACHINE
	}

	return registry.CURRENT_USER
}

func (w *windowsAutostart) writeRunKey(elevated bool) error {
	return runKeys.set(runKeyRoot(elevated), w.name, w.target)
}

func (w *windowsAutostart) deleteRunKey(elevated bool) error {
	return runKeys.del(runKeyRoot(elevated), w.name)
}

func (w *windowsAutostart) runKeyStatus(elevated bool) (autostartEntry, error) {
	scope := "user"
	if elevated {
		scope = "machine"
	}

	e := autostartEntry{Method: methodStartup, Scope: scope}

	v, present, err := runKeys.get(runKeyRoot(elevated), w.name)
	if err != nil {
		return e, err
	}

	if present {
		e.Enabled = true
		e.Target = v
	}

	return e, nil
}

// createTask registers an ONLOGON scheduled task. With elevated set it runs as
// SYSTEM with highest privileges — the same shape as Google's machine-wide
// updater task, and Syncthing's "Run with highest privileges" option.
func (w *windowsAutostart) createTask(elevated bool) error {
	args := []string{
		"/Create", "/TN", w.name,
		"/TR", w.target,
		"/SC", "ONLOGON",
		"/F",
	}
	if elevated {
		args = append(args, "/RU", "SYSTEM", "/RL", "HIGHEST")
	}

	return runSchtasksFn(args)
}

func (w *windowsAutostart) deleteTask() error {
	return runSchtasksFn([]string{"/Delete", "/TN", w.name, "/F"})
}

func (w *windowsAutostart) taskStatus() (autostartEntry, error) {
	e := autostartEntry{Method: methodTaskScheduler, Scope: "logon", Target: w.target}

	// queryTaskFn reports false when the task is absent (the "not enabled" signal);
	// a non-nil error means the query itself failed unexpectedly (e.g. schtasks.exe
	// missing) and must be surfaced, not reported as "not enabled".
	present, err := queryTaskFn(w.name)
	if err != nil {
		e.Target = ""

		return e, err
	}

	if present {
		e.Enabled = true

		return e, nil
	}

	e.Target = ""

	return e, nil
}

// winRunKeyStore is the production runKeyStore backed by the Windows registry.
type winRunKeyStore struct{}

func (winRunKeyStore) set(root registry.Key, name, value string) error {
	k, err := registry.OpenKey(root, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}

	defer func() { _ = k.Close() }()

	if err := k.SetStringValue(name, value); err != nil {
		return fmt.Errorf("set Run value: %w", err)
	}

	return nil
}

func (winRunKeyStore) del(root registry.Key, name string) error {
	k, err := registry.OpenKey(root, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}

	defer func() { _ = k.Close() }()

	// Deleting an already-absent value is success (idempotent disable).
	if err := k.DeleteValue(name); err != nil && !errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
		return fmt.Errorf("delete Run value: %w", err)
	}

	return nil
}

func (winRunKeyStore) get(root registry.Key, name string) (string, bool, error) {
	k, err := registry.OpenKey(root, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", false, nil // the Run key itself is absent — simply not enabled
		}

		return "", false, fmt.Errorf("open Run key: %w", err)
	}

	defer func() { _ = k.Close() }()

	v, _, err := k.GetStringValue(name)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", false, nil // value absent — not enabled
		}

		return "", false, fmt.Errorf("get Run value: %w", err)
	}

	return v, true, nil
}

// queryTask reports whether a scheduled task with the given name exists. A non-zero
// schtasks exit (an *exec.ExitError) means the task is absent — reported as (false,
// nil). Any error that prevents schtasks from running at all (e.g. the binary is
// missing) is surfaced so Status does not mask it as "not enabled".
func queryTask(name string) (bool, error) {
	err := exec.Command("schtasks.exe", "/Query", "/TN", name).Run()
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}

	return false, fmt.Errorf("schtasks /Query %s: %w", name, err)
}

// runSchtasks invokes schtasks.exe and folds its output into any error so the
// caller sees why the OS refused (e.g. "Access is denied" without admin).
func runSchtasks(args []string) error {
	out, err := exec.Command("schtasks.exe", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
	}

	return nil
}
