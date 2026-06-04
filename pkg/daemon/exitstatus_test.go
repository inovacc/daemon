package daemon

import (
	"errors"
	"fmt"
	"testing"
)

func TestExitNeedsPrivilegeValue(t *testing.T) {
	if ExitNeedsPrivilege != 5 {
		t.Fatalf("ExitNeedsPrivilege = %d, want 5", ExitNeedsPrivilege)
	}
	if ExitNeedsPrivilege.AsInt() != 5 {
		t.Fatalf("AsInt() = %d, want 5", ExitNeedsPrivilege.AsInt())
	}
}

func TestExitCodesAreDistinct(t *testing.T) {
	seen := map[int]ExitStatus{}
	for _, e := range []ExitStatus{ExitSuccess, ExitError, ExitRestart, ExitUpgrade, ExitNeedsPrivilege} {
		if prev, ok := seen[e.AsInt()]; ok {
			t.Fatalf("exit code %d reused by %d and %d", e.AsInt(), prev, e)
		}
		seen[e.AsInt()] = e
	}
}

func TestExitCodeForMapsPrivilege(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"privilege sentinel", ErrNeedsPrivilege, int(ExitNeedsPrivilege)},
		{"wrapped privilege sentinel", fmt.Errorf("svc install: %w", ErrNeedsPrivilege), int(ExitNeedsPrivilege)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExitCodeFor(tc.err); got != tc.want {
				t.Fatalf("ExitCodeFor(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

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
