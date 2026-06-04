package daemon

// isElevatedFn is the test seam fronting the platform isElevated implementation
// (elevate_windows.go / elevate_unix.go). Production points it at isElevated; tests
// override it to drive the privileged/unprivileged branches of RequirePrivilege.
var isElevatedFn = isElevated
