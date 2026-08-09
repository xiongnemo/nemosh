//go:build !windows

package runtime

// A POSIX filesystem accepts any byte but NUL in a name, and a NUL cannot reach
// here through a Go string used as a path, so there is no such class off
// Windows.
func isUnusableName(error) bool { return false }
