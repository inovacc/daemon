package daemon

import (
	"errors"
	"testing"
)

func TestExitCodeForNilIsSuccess(t *testing.T) {
	if got := ExitCodeFor(nil); got != ExitSuccess.AsInt() {
		t.Fatalf("ExitCodeFor(nil) = %d, want %d", got, ExitSuccess.AsInt())
	}
}

func TestExitCodeForGenericErrorIsExitError(t *testing.T) {
	if got := ExitCodeFor(errors.New("boom")); got != ExitError.AsInt() {
		t.Fatalf("ExitCodeFor(err) = %d, want %d", got, ExitError.AsInt())
	}
}
