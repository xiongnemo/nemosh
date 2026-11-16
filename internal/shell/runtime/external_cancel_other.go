//go:build !windows

package runtime

// childSharesConsole is the Windows console's question, which nothing asks here: an interrupt
// is never the console's off Windows, so the program is ended as it always was.
var childSharesConsole = func(int) bool { return false }
