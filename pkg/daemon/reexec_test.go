package daemon

import (
	"reflect"
	"testing"
)

// TestReexecFnDefaultsToReal asserts the seam is wired to the real platform
// implementation by default, mirroring spawnDetachedFn / stopProcessFn.
func TestReexecFnDefaultsToReal(t *testing.T) {
	if reexecFn == nil {
		t.Fatal("reexecFn must default to a non-nil implementation")
	}

	want := reflect.ValueOf(reexecSelf).Pointer()

	got := reflect.ValueOf(reexecFn).Pointer()
	if got != want {
		t.Fatalf("reexecFn must default to reexecSelf")
	}
}
