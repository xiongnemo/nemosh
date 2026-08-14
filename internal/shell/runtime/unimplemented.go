package runtime

import "fmt"

// unimplementedBuiltin names a shell builtin Nemosh recognises and does not
// implement, and says why.
//
// Without this they fell through to command lookup and came back as
// `fg: not found` with status 127 — the same words a missing external program
// gets, so a reader could not tell "this shell does not have it" from "install
// it". Where busybox-w32 does not implement it either, the reason says so,
// because that is the difference between a gap in Nemosh and a limit of the
// platform.
//
// Status 126 rather than 127: SUSv3 keeps 127 for a command that could not be
// found and 126 for one that was found and could not be run, which is exactly
// the distinction being drawn.
type unimplementedBuiltin struct {
	reason string
}

const unimplementedBuiltinStatus = 126

var unimplementedBuiltins = map[string]unimplementedBuiltin{
	"hash": {
		reason: "command lookup is not cached, so there is nothing to remember or forget. " +
			"busybox-w32 does implement it, over a hash table this shell does not have",
	},
	"ulimit": {
		reason: "Windows has no getrlimit. busybox-w32 does not implement it either: " +
			"it keeps the name and returns 1 with no message (shell/shell_common.c, " +
			"the #else of `#if !ENABLE_PLATFORM_MINGW32`)",
	},
	"fg": {reason: noSuspensionReason},
	"bg": {reason: noSuspensionReason},
}

// noSuspensionReason is why fg and bg are refused, and it is deliberately not
// "Windows has no process groups".
//
// That is true and it is the second reason. The first is that there is nothing to
// resume: `fg` and `bg` continue a job that was *suspended*, and no layer under
// this shell can suspend one. Naming the process group instead would suggest the
// gap is about terminal ownership, and someone would reasonably try to close it.
//
// The contrast with `kill` is the whole of it. Ending something maps cleanly onto
// cancelling its context -- both are one-way doors, so `kill %N` is honest.
// Ctrl-Z needs a door that opens both ways, and there is not one: Go parks a
// goroutine only when the goroutine itself blocks, with no API to freeze one from
// outside, and Windows has no SIGSTOP for a real process either.
const noSuspensionReason = "a job cannot be suspended, and fg and bg resume a suspended job. " +
	"`kill %N` works because ending a job maps onto cancelling its context, which is one-way; " +
	"suspension needs stop-and-continue, and neither layer below offers it -- Go cannot freeze a " +
	"goroutine from outside, and Windows has no SIGSTOP even for a real process. " +
	"busybox-w32 reaches the same conclusion: JOBS is 0 under ENABLE_PLATFORM_MINGW32 " +
	"(shell/ash.c:247-253), its own comment there says the Windows build \"doesn't enable job " +
	"control, just some job-related features\", and no SIGSTOP, SIGTSTP or SIGCONT appears " +
	"anywhere in its win32 layer"

// reportUnimplementedBuiltin prints the refusal and reports whether the name was
// one. Checked before applet lookup and before PATH, so a program that happens
// to share the name cannot make the answer depend on what is installed.
func (r Runtime) reportUnimplementedBuiltin(name string) (int, bool) {
	builtin, ok := unimplementedBuiltins[name]
	if !ok {
		return 0, false
	}
	fmt.Fprintf(r.streams.Stderr, "%s: not implemented: %s\n", name, builtin.reason)
	return unimplementedBuiltinStatus, true
}
