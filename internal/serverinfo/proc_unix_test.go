//go:build !windows

package serverinfo

import (
	"os"
	"testing"
)

func TestProcessAliveUnix(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Error("the current process must report alive")
	}

	if processAlive(0) || processAlive(-1) {
		t.Error("non-positive pids must not be alive")
	}

	// A pid far above the platform maximum is not a live process (signal 0 -> ESRCH).
	if processAlive(1 << 30) {
		t.Error("a bogus high pid must not report alive")
	}
}
