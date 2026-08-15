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
// permanent separates "not yet" from "never", because they are different
// answers and a reader acts on them differently. `hash` could be implemented any
// day -- it is a cache this shell has not got round to. `fg` cannot be, and
// someone who reads "not implemented" may reasonably go and try; saying so
// spares them.
type unimplementedBuiltin struct {
	reason    string
	permanent bool
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
	"fg": {reason: noSuspensionReason, permanent: true},
	"bg": {reason: noSuspensionReason, permanent: true},
}

// noSuspensionReason is why fg and bg are refused.
//
// One sentence, on purpose. This is read at a prompt by someone who typed two
// letters, and the earlier version was a paragraph citing three platform layers
// and a busybox line number -- true, and not what anyone wants back from `fg`.
// The full argument lives in docs/support-matrix.md under Process control, which
// is where a reader who actually wants it will look.
//
// It is deliberately not "Windows has no process groups", which is true and is
// the second reason. The first is that there is nothing to resume, and that is
// also what makes `kill %N` fine by contrast: ending a job maps onto cancelling
// its context, and both are one-way.
const noSuspensionReason = "nothing here can suspend a job, so there is nothing for them to resume " +
	"(`kill %N` still works). See docs/support-matrix.md, Process control"

// reportUnimplementedBuiltin prints the refusal and reports whether the name was
// one. Checked before applet lookup and before PATH, so a program that happens
// to share the name cannot make the answer depend on what is installed.
func (r Runtime) reportUnimplementedBuiltin(name string) (int, bool) {
	builtin, ok := unimplementedBuiltins[name]
	if !ok {
		return 0, false
	}
	// "and will not be" is the whole point of the distinction: a reader told only
	// "not implemented" may go looking for a flag, a newer build, or an issue to
	// file. One that is settled should say it is settled.
	verdict := "not implemented"
	if builtin.permanent {
		verdict = "not implemented, and will not be"
	}
	fmt.Fprintf(r.streams.Stderr, "%s: %s: %s\n", name, verdict, builtin.reason)
	return unimplementedBuiltinStatus, true
}
