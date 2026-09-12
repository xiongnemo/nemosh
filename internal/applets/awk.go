package applets

import (
	"context"
	"io"
)

// awk, the pattern-scanning language.
//
// The applet is the last piece rather than the first: until the language could run real
// programs the name stayed unregistered, so that `awk` on this shell's PATH kept finding
// whatever the user already had. master is their nightly, and a half-finished awk shadowing
// a working one would have made their scripts worse every day it took to finish.
//
// **It is a named type rather than a simpleApplet holding a closure, and that is
// load-bearing.** awk dispatches into the applet registry for `system`, `print | cmd` and
// `cmd | getline`, and awk is itself in that registry -- so a constructor that referred to
// the run function would make the package's initialisation cycle back on itself
// (DefaultRegistry -> newAwkApplet -> ... -> DefaultRegistry), which Go refuses to compile.
// Returning a zero value of a type whose method does the work breaks the reference chain.
// `xargs` is here for the same reason and was the precedent.
//
// What differs from other awks is written down in docs/support-matrix.md; two things are
// worth knowing at the top:
//
//   - **A command is an applet.** Anything else is refused by name, because
//     internal/applets does not spawn OS processes. See awk_command.go.
//   - **Text is counted in runes.** `length("héllo")` is 5, and `substr`, `index` and
//     `match` agree with it. See awk_builtin.go for why.
type awkApplet struct{}

func newAwkApplet() Applet { return awkApplet{} }

func (awkApplet) Name() string { return "awk" }

// Run parses the command line, then the program, then runs it.
//
// A failure exits **2**, which is gawk's status and what lets a script tell a broken
// program from one that chose to `exit 1`.
func (awkApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	invocation, err := parseAwkArguments(args)
	if err != nil {
		return ExitStatusMessage(2, err)
	}
	program, err := parseAwkProgram(invocation.program)
	if err != nil {
		return ExitStatusMessage(2, err)
	}
	// Standard input is decoded the way every text applet decodes its input, so a UTF-16
	// pipe is text rather than interleaved NULs.
	status, err := runAwkProgram(ctx, program, invocation, decodeTextInput(stdin), stdout, stderr)
	if err != nil {
		return ExitStatusMessage(2, err)
	}
	if status != 0 {
		// An `exit n` chose this, so it carries no diagnostic: the program already said
		// whatever it meant to say.
		return ExitStatus(status)
	}
	return nil
}
