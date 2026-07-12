//go:build windows

package serverinfo

import (
	"math"
	"os"
	"testing"
)

func TestProcessAliveWindows(t *testing.T) {
	if !processAlive(os.Getpid()) {
		t.Error("the current process must report alive")
	}

	if processAlive(0) || processAlive(-1) {
		t.Error("non-positive pids must not be alive")
	}

	// Above the uint32 range: the overflow guard must reject it, not wrap.
	if processAlive(math.MaxUint32 + 1) {
		t.Error("a pid above the uint32 range must not be alive")
	}

	// A very high in-range pid is almost certainly dead.
	if processAlive(1 << 30) {
		t.Error("a bogus high pid must not report alive")
	}
}
