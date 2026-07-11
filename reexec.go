package daemon

// reexecFn is a seam (mirrors spawnDetachedFn / stopProcessFn) overridden in tests;
// production points at the platform implementation in reexec_unix.go / reexec_windows.go.
// On success it does not return (the process image is replaced or exits).
//
// This shared declaration lives in reexec.go — alongside the platform-tagged
// reexec_unix.go / reexec_windows.go — so all four launch/stop seams follow one
// pattern (shared var in <name>.go, impls in <name>_<os>.go).
var reexecFn = reexecSelf
