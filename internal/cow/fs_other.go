//go:build !linux && !darwin

package cow

// Copy-on-write workspaces are only supported on Linux and macOS; on any other
// platform Clone refuses before these are ever consulted.

func deviceID(string) (uint64, bool) { return 0, false }

func filesystemName(string) string { return "" }
