//go:build !windows

package runtime

// Case resolution is a Windows concern: a POSIX filesystem is case-sensitive,
// so the spelling that opened a path is already the spelling on disk.
func realCaseNativePath(native string) string { return native }
