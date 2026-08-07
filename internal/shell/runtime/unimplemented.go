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
	"fg": {
		reason: "job control needs a terminal process group, which Windows does not have. " +
			"busybox-w32 does not implement it either: fg and bg are compiled out there " +
			"(`#if JOBS`, and JOBS is 0 under ENABLE_PLATFORM_MINGW32, shell/ash.c:246-252)",
	},
	"bg": {
		reason: "job control needs a terminal process group, which Windows does not have. " +
			"busybox-w32 does not implement it either: fg and bg are compiled out there " +
			"(`#if JOBS`, and JOBS is 0 under ENABLE_PLATFORM_MINGW32, shell/ash.c:246-252)",
	},
}

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
