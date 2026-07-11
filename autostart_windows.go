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

// windowsAutostart implements autostartManager against the registry Run key
// (Startup method) and schtasks.exe (Task Scheduler method).
type windowsAutostart struct {
	name   string // registry value name / scheduled-task name (Options.ServiceName)
	target string // full command line the entry launches: "<exe>" service
}

// realAutostartManager builds the Windows autostart manager, resolving the target
// command line once from the running executable and the launch argv.
func realAutostartManager(o Options, launch []string) (autostartManager, error) {
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
	return []autostartEntry{
		w.runKeyStatus(false),
		w.runKeyStatus(true),
		w.taskStatus(),
	}, nil
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
	k, err := registry.OpenKey(runKeyRoot(elevated), runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}
	defer func() { _ = k.Close() }()

	if err := k.SetStringValue(w.name, w.target); err != nil {
		return fmt.Errorf("set Run value: %w", err)
	}

	return nil
}

func (w *windowsAutostart) deleteRunKey(elevated bool) error {
	k, err := registry.OpenKey(runKeyRoot(elevated), runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}
	defer func() { _ = k.Close() }()

	// Deleting an already-absent value is success (idempotent disable).
	if err := k.DeleteValue(w.name); err != nil && !errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
		return fmt.Errorf("delete Run value: %w", err)
	}

	return nil
}

func (w *windowsAutostart) runKeyStatus(elevated bool) autostartEntry {
	scope := "user"
	if elevated {
		scope = "machine"
	}

	e := autostartEntry{Method: methodStartup, Scope: scope}

	k, err := registry.OpenKey(runKeyRoot(elevated), runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return e
	}
	defer func() { _ = k.Close() }()

	if v, _, err := k.GetStringValue(w.name); err == nil {
		e.Enabled = true
		e.Target = v
	}

	return e
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

	return runSchtasks(args)
}

func (w *windowsAutostart) deleteTask() error {
	return runSchtasks([]string{"/Delete", "/TN", w.name, "/F"})
}

func (w *windowsAutostart) taskStatus() autostartEntry {
	e := autostartEntry{Method: methodTaskScheduler, Scope: "logon", Target: w.target}

	// /Query exits non-zero when the task is absent; that is the "not enabled"
	// signal, not an error to surface.
	if err := exec.Command("schtasks.exe", "/Query", "/TN", w.name).Run(); err != nil {
		e.Enabled = false
		e.Target = ""

		return e
	}

	e.Enabled = true

	return e
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
