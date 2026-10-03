package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// These serve nemosh's own command line, `nemosh -eu -o pipefail script`, which a
// `#!/bin/sh -e` line produces too: the launch rewrites it to `nemosh -e script`
// (external_script.go). The letters mean what `set` makes them mean, so they go through
// the same code and are refused for the same reasons.

// ErrUnknownOption is an option letter or name the shell does not have, as against one it
// has and refuses to turn on.
var ErrUnknownOption = errors.New("illegal option")

// SetOption turns a shell option on or off as `set` would: by letter, or by its `-o` name
// when letter is zero. The error is `set`'s diagnostic without the `set:` in front.
func (r Runtime) SetOption(letter byte, name string, enable bool) error {
	if letter != 0 {
		return r.setOptionLetter(letter, enable)
	}
	return r.setOptionName(name, enable)
}

// SetInvocationMode records how the shell was started, which `$-` reports after the
// options: c for a command string, s for commands read from standard input, i for an
// interactive session. `$-` said none of it, so `case $- in *i*)`, the usual way for a
// startup file to learn whether it is interactive, answered no at a prompt.
//
// A session also has bash's histexpand, history and emacs on, which a script has off.
func (r Runtime) SetInvocationMode(letters string) {
	r.options.invocation = letters
	if strings.ContainsRune(letters, 'i') {
		r.options.histExpand, r.options.history, r.options.emacs = true, true, true
		// The characters Tab's word ends at, bash's, which a completion script reads to know
		// what the word it is given lacks; see complete_line.go. A session's alone: a script
		// completes nothing.
		if _, set := r.vars["COMP_WORDBREAKS"]; !set {
			r.vars["COMP_WORDBREAKS"] = defaultCompletionWordBreaks
		}
	}
}

// HistoryExpansion is `set -o histexpand`, -H, which a session asks before it expands `!`.
func (r Runtime) HistoryExpansion() bool { return r.options.histExpand }

// HistoryRecording is `set -o history`, which a session asks before it keeps a line.
func (r Runtime) HistoryRecording() bool { return r.options.history }

// MarkLoginShell records that the shell was started as a login shell, which `shopt
// login_shell` reports. Neither reference puts it in `$-`.
func (r Runtime) MarkLoginShell() { r.options.login = true }

// ListOptions prints what `set -o` prints, which is what `nemosh -o` with no name after
// it does in both references.
func (r Runtime) ListOptions() { r.listShellOptions(true) }

// SetShopt is `shopt -s NAME`, or `shopt -u NAME` when enable is false, for nemosh's own
// -O NAME and +O NAME, which are bash's. busybox has neither.
func (r Runtime) SetShopt(name string, enable bool) error { return r.setShopt(name, enable) }

// ListShopts prints what `shopt` prints, or with reusable what `shopt -p` prints: bash's
// -O and +O with no name after them.
func (r Runtime) ListShopts(reusable bool) { r.listShopts(shoptRequest{print: reusable}, nil) }

// SourceStartup runs a startup file -- $ENV, or a login shell's profiles -- as part of the
// shell rather than as a script of its own. Its EXIT trap is the shell's, left for when the
// shell exits. An `exit` in it is the shell's exit, which the second answer reports, and
// the caller ends the shell with CloseBatch as a script's end would. A file that does not
// parse is the error, unreported, so the caller can name the file.
//
// These went through RunScript, which ran a profile's `trap ... EXIT` the moment the
// profile ended, and let `exit 3` end only the profile. busybox runs the trap at the
// shell's exit, and exits 3.
func (r Runtime) SourceStartup(ctx context.Context, script string) (int, bool, error) {
	prepared, err := ParseScript(script)
	if err != nil {
		return 2, false, err
	}
	r.sayParseWarnings(prepared)
	control := flowNone
	status := r.guardedStatus("running a startup file", func() int {
		status, flow := r.executeRead(ctx, prepared.program, 0)
		control = flow
		return status
	})
	switch control {
	case flowExec:
		r.lifecycle.exitSuppressed = true
		return status, true, nil
	case flowExit:
		return status, true, nil
	}
	return status, false, nil
}

// CheckSyntax is `nemosh -n`: the script is parsed and none of it runs. 0 when it parses,
// and 2 with the parser's diagnostic when it does not -- the status and the words a run
// would have stopped with, because a run parses the whole script before starting it.
func (r Runtime) CheckSyntax(script string) int {
	prepared, err := ParseScript(script)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s: %v\n", r.diagnosticName(), err)
		return 2
	}
	r.sayParseWarnings(prepared)
	return 0
}
