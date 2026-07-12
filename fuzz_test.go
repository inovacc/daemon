package daemon

import (
	"strings"
	"testing"
)

// FuzzValidateServiceName asserts the accept/reject invariant: whatever the input,
// validateServiceName must not panic, and any name it ACCEPTS must be non-empty and
// contain only the allowed charset (so it is always safe as a task/registry/service id).
func FuzzValidateServiceName(f *testing.F) {
	for _, s := range []string{"", "ok", "my-app_1.2", "bad name!", "ünïcode", "/etc/passwd", strings.Repeat("a", 300)} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, name string) {
		if err := validateServiceName(name); err != nil {
			return // rejected — nothing more to assert
		}

		if strings.TrimSpace(name) == "" {
			t.Fatalf("accepted empty/whitespace name %q", name)
		}

		for _, r := range name {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			default:
				t.Fatalf("accepted name %q containing disallowed rune %q", name, r)
			}
		}
	})
}

// FuzzValidatePort asserts validatePort errors exactly when the port is out of the
// 0-65535 range, and never panics.
func FuzzValidatePort(f *testing.F) {
	for _, p := range []int{-1, 0, 1, 80, 9500, 65535, 65536, 1 << 30} {
		f.Add(p)
	}

	f.Fuzz(func(t *testing.T, port int) {
		outOfRange := port < 0 || port > 65535
		if got := validatePort("p", port) != nil; got != outOfRange {
			t.Fatalf("validatePort(%d): err=%v, want out-of-range=%v", port, got, outOfRange)
		}
	})
}

// FuzzChildEnvName asserts the recursion-guard var is always a valid environment
// identifier ([A-Z0-9_]), non-empty, and deterministic for any binary name.
func FuzzChildEnvName(f *testing.F) {
	for _, s := range []string{"", "app", "my-app", "MY_APP", "a b c", "日本語", "x!@#$"} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, name string) {
		got := childEnvName(name)
		if got == "" {
			t.Fatalf("childEnvName(%q) is empty", name)
		}

		if got != childEnvName(name) {
			t.Fatalf("childEnvName(%q) is not deterministic", name)
		}

		for _, r := range got {
			if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
				t.Fatalf("childEnvName(%q)=%q has a non-identifier rune %q", name, got, r)
			}
		}
	})
}
