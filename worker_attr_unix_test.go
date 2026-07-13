//go:build !windows

package daemon

import "testing"

// F1: the worker gets its own process group (Setpgid) so a signal can target it
// (and any children it spawns) without also hitting the monitor.
func TestWorkerSysProcAttr(t *testing.T) {
	attr := workerSysProcAttr()
	if attr == nil {
		t.Fatal("workerSysProcAttr returned nil")
	}

	if !attr.Setpgid {
		t.Error("Setpgid must be true so the worker gets its own process group")
	}
}
