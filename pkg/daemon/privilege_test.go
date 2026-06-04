package daemon

import (
	"bytes"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// buildPrivPath constructs a realistic command tree so cmd.CommandPath() returns
// "daemon svc install" — the exact string RequirePrivilege embeds in its guidance.
func buildPrivPath(t *testing.T, verb string) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "daemon"}
	svc := &cobra.Command{Use: "svc"}
	leaf := &cobra.Command{Use: verb}
	svc.AddCommand(leaf)
	root.AddCommand(svc)
	return leaf
}

func TestRequirePrivilegeBlocksWhenNotElevated(t *testing.T) {
	orig := isElevatedFn
	t.Cleanup(func() { isElevatedFn = orig })
	isElevatedFn = func() bool { return false }

	leaf := buildPrivPath(t, "install")
	var stderr bytes.Buffer
	leaf.SetErr(&stderr)

	err := RequirePrivilege(leaf)
	if !errors.Is(err, ErrNeedsPrivilege) {
		t.Fatalf("err = %v, want ErrNeedsPrivilege", err)
	}
	out := stderr.String()
	// Guidance must reference the full command path so the user can copy-paste the re-run.
	if !strings.Contains(out, "daemon svc install") {
		t.Fatalf("guidance missing command path %q: %q", "daemon svc install", out)
	}
	low := strings.ToLower(out)
	switch runtime.GOOS {
	case "windows":
		if !strings.Contains(low, "runas") && !strings.Contains(low, "administrator") {
			t.Fatalf("windows guidance missing RunAs/administrator hint: %q", out)
		}
	default:
		if !strings.Contains(low, "sudo") {
			t.Fatalf("unix guidance missing sudo hint: %q", out)
		}
	}
}

func TestRequirePrivilegeProceedsWhenElevated(t *testing.T) {
	orig := isElevatedFn
	t.Cleanup(func() { isElevatedFn = orig })
	isElevatedFn = func() bool { return true }

	leaf := buildPrivPath(t, "install")
	var stderr bytes.Buffer
	leaf.SetErr(&stderr)

	if err := RequirePrivilege(leaf); err != nil {
		t.Fatalf("RequirePrivilege returned %v, want nil when elevated", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("no guidance expected when elevated, got %q", stderr.String())
	}
}
