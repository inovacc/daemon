//go:build windows

package daemon

import (
	"testing"

	"golang.org/x/sys/windows"
)

// F1: the worker must get CREATE_NO_WINDOW (else Windows allocates a phantom
// console for a child of the console-less monitor) and CREATE_NEW_PROCESS_GROUP.
// It must NOT get DETACHED_PROCESS — unlike detachedSysProcAttr, the worker must
// stay a real child of the monitor so exec.CommandContext can Wait() on it and
// `taskkill /T` still reaps the tree.
func TestWorkerSysProcAttr(t *testing.T) {
	attr := workerSysProcAttr()
	if attr == nil {
		t.Fatal("workerSysProcAttr returned nil")
	}

	if !attr.HideWindow {
		t.Error("HideWindow must be true so the worker never shows a window")
	}

	if attr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Error("CreationFlags must include CREATE_NO_WINDOW")
	}

	if attr.CreationFlags&windows.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Error("CreationFlags must include CREATE_NEW_PROCESS_GROUP")
	}

	if attr.CreationFlags&windows.DETACHED_PROCESS != 0 {
		t.Error("CreationFlags must NOT include DETACHED_PROCESS: the worker must stay a child of the monitor")
	}
}
