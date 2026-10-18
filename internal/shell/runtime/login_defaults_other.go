//go:build !windows

package runtime

// Nothing to fill in here. A Unix login sets HOME, USER, LOGNAME and SHELL
// before any shell starts, so a shell that found them missing would be running
// somewhere deliberately bare, and inventing values would hide that rather than
// help.
func loginDefaults(pathState) map[string]string { return nil }
