package daemon

import (
	"slices"
	"testing"
)

// TestDefaultLaunchArgs_Daemonizes pins the standalone autostart entry to the
// DAEMONIZING command ("service start"), not the foreground monitor ("service").
//
// Why this matters (regression guard):
//
// A logon autostart entry is launched by explorer.exe, which has NO console.
// Windows therefore allocates a fresh console for a console-subsystem child, so
// a bare "<exe> service" — which hosts the monitor IN the launched process —
// pops a visible PseudoConsoleWindow in the user's face on every single login.
// Confirmed in the field via EnumWindows: the launched "service" process owned a
// visible window of class PseudoConsoleWindow; "service start" owned none,
// because it daemonizes (spawnDetached => DETACHED_PROCESS|CREATE_NO_WINDOW)
// and exits.
//
// If someone "simplifies" defaultLaunchArgs back to just the verb, the console
// window comes back and nothing else in the suite would notice — hence this test.
func TestDefaultLaunchArgs_Daemonizes(t *testing.T) {
	want := []string{"service", "start"}
	if !slices.Equal(defaultLaunchArgs, want) {
		t.Fatalf("defaultLaunchArgs = %q, want %q\n"+
			"a standalone autostart entry MUST daemonize; the bare %q verb runs the "+
			"monitor in the launched process and pops a console window at logon",
			defaultLaunchArgs, want, autostartVerb)
	}

	// Guard the invariant directly rather than only the literal: the entry must
	// carry the start subcommand, or it does not detach.
	if !slices.Contains(defaultLaunchArgs, autostartStartVerb) {
		t.Fatalf("defaultLaunchArgs %q is missing the %q subcommand — it will not "+
			"detach, and will show a console window at logon",
			defaultLaunchArgs, autostartStartVerb)
	}
}
