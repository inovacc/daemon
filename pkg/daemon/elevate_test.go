package daemon

import "testing"

func TestIsElevatedFnDefaultsToIsElevated(t *testing.T) {
	// The seam must default to the platform implementation and be callable on every
	// supported OS without panicking. We assert only that it returns (no panic); the
	// concrete bool depends on how the test process was launched.
	_ = isElevatedFn()
}

func TestIsElevatedFnIsOverridable(t *testing.T) {
	orig := isElevatedFn

	t.Cleanup(func() { isElevatedFn = orig })

	isElevatedFn = func() bool { return true }
	if !isElevatedFn() {
		t.Fatal("override to true failed")
	}

	isElevatedFn = func() bool { return false }
	if isElevatedFn() {
		t.Fatal("override to false failed")
	}
}
