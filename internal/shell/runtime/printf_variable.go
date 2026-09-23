package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// appletStatus is the status an applet's run answers with, and the diagnostic it
// leaves: the same whether the applet wrote to the terminal or into a variable.
func (r Runtime) appletStatus(ctx context.Context, name string, err error) int {
	if err == nil {
		return 0
	}
	if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		return contextStatus(ctx)
	}
	// Normalized first, which is what this was missing: an applet returns the raw
	// *fs.PathError from the failed write, and that is not the sentinel. external.go
	// already normalizes on its own path; this one compared against the sentinel and
	// never matched, so every `producer | head -1` where the producer was an applet
	// reported a write failure that POSIX would have passed over in silence.
	if errors.Is(normalizePipelineWriteError(err), errPipelineDownstreamClosed) {
		return 0
	}
	status, message := AppletFailure(name, err)
	if message != "" {
		fmt.Fprintln(r.streams.Stderr, message)
	}
	return status
}

// printfToVariable is `printf -v name format args...`: bash's way of formatting into a
// variable without the subshell `$(printf ...)` costs, and so the way a loop that
// formats every line is written. printf is an applet, which cannot reach the shell's
// variables, so it was handed `-v` as its format and printed `-v`, with status 0 and
// the variable untouched.
//
// The applet does the formatting -- one printf, so the two cannot drift -- into a
// buffer, and the result is assigned whole: trailing newlines kept, which is where it
// differs from `$(...)`, and an element when the name has a subscript. A conversion
// error still assigns what was produced, with printf's status, as bash does. busybox's
// printf has no -v, so bash is the reference throughout.
func (r Runtime) printfToVariable(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.streams.Stderr, "printf: -v: option requires an argument")
		return 2
	}
	name, rest := args[0], args[1:]
	if len(rest) > 0 && rest[0] == "--" {
		rest = rest[1:]
	}
	if len(rest) == 0 {
		fmt.Fprintln(r.streams.Stderr, "printf: usage: printf [-v var] format [arguments]")
		return 2
	}
	base := name
	if reference, ok := parseArrayReference(name); ok {
		base = reference.name
	} else if !isVariableName(name) {
		fmt.Fprintf(r.streams.Stderr, "printf: `%s': not a valid identifier\n", name)
		return 2
	}
	if r.isReadonly(base) {
		// printf is an ordinary utility, so this refuses and the script goes on --
		// the same as `read` into a readonly name, and as bash answers.
		fmt.Fprintf(r.streams.Stderr, "printf: %s: readonly variable\n", base)
		return 1
	}
	applet, ok := r.lookupApplet("printf")
	if !ok {
		fmt.Fprintln(r.streams.Stderr, "printf: not available in this build")
		return 127
	}
	var formatted strings.Builder
	err := applet.Run(applets.WithProcessView(ctx, r), rest, r.streams.Stdin, &formatted, r.streams.Stderr)
	status := r.appletStatus(ctx, "printf", err)
	if assigned := r.assignVar(name, formatted.String()); assigned != 0 {
		return assigned
	}
	return status
}
