// Package proc is the shell's view of other processes: how to list them and how
// to stop them.
//
// It exists to be the only copy. Two callers need the same two operations from
// opposite sides of the tree -- the `kill` builtin, because `%N` names a job and
// only the shell has the job table, and the `pgrep`/`pkill` applets, which know
// nothing about jobs. The runtime imports applets, so applets cannot import the
// runtime, and without a package in between one of them would have grown a second
// TerminateProcess. Two copies of a lookup drift; this project has already fixed
// that once, in `command -v`.
//
// Windows is where this is implemented. busybox does the same work in the same
// places: CreateToolhelp32Snapshot for the process list (win32/process.c:740) and
// TerminateProcess for the kill (win32/process.c:909). Elsewhere the platform's
// own signal is used for killing, and listing is refused rather than guessed at
// -- reading /proc would work on Linux and nothing portable works on macOS, and
// Linux here is a build-and-test target rather than a supported one.
package proc

import "errors"

// Process is one entry of the process list.
type Process struct {
	PID int
	// Name is the executable's file name, which is what a pattern is matched
	// against. Not the full command line: getting that on Windows means opening
	// each process and reading its PEB, which needs privileges an ordinary
	// session has not got for anything it does not own.
	Name string
}

// ErrListUnsupported is returned where the process list cannot be read. A caller
// must report it rather than treat it as an empty list -- "nothing matched" and
// "I cannot see" are different answers, and only one of them is safe to act on.
var ErrListUnsupported = errors.New("listing processes is not implemented on this platform")
