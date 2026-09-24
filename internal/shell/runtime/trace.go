package runtime

import (
	"context"
	"fmt"
	"strings"
)

// tracePrefix is what `set -x` writes before each command. POSIX says the
// prefix comes from PS4 and defaults to "+ ".
const defaultTracePrefix = "+ "

// traceCommand is `set -x`: write the command about to run to stderr, after
// expansion, so what is shown is what will actually happen. Quoting is added
// where a word would not survive being read back, which is what makes the trace
// of an empty or space-bearing argument readable.
func (r Runtime) traceCommand(ctx context.Context, args []string, savedStatus int) {
	if !r.options.xtrace || len(args) == 0 {
		return
	}
	prefix := defaultTracePrefix
	if custom, ok := r.vars["PS4"]; ok {
		prefix = r.tracePrefix(ctx, custom, savedStatus)
	}
	assignments, _ := leadingAssignments(args)
	quoted := make([]string, len(args))
	for index, arg := range args {
		if index < len(assignments) {
			quoted[index] = traceAssignment(arg)
			continue
		}
		quoted[index] = traceWord(arg)
	}
	fmt.Fprintf(r.streams.Stderr, "%s%s\n", prefix, strings.Join(quoted, " "))
}

// tracePrefix expands PS4, as POSIX asks and both references do: `PS4='+$LINENO: '` is
// how a trace says where each command is, and it was printed as written. The trace is off
// while it expands, or a PS4 that runs a command would trace that command, and that trace
// would expand PS4 again. What the expansion leaves behind is put back, so tracing a
// command cannot change its status: an assignment's status is its last substitution's.
func (r Runtime) tracePrefix(ctx context.Context, ps4 string, savedStatus int) string {
	state := *r.expansion
	r.options.xtrace = false
	defer func() {
		r.options.xtrace = true
		r.expansion.shellError = state.shellError
		r.expansion.substitutionStatus, r.expansion.substitutions = state.substitutionStatus, state.substitutions
	}()
	return r.ExpandPromptString(ctx, ps4, savedStatus)
}

// traceAssignment shows `name=value` with only the value quoted, busybox's spelling:
// `y='a b'`, where quoting the whole word gave `'y=a b'`. An assignment-only command was
// not traced at all, so a trace could not show where a variable got its value; both
// references trace it.
func traceAssignment(arg string) string {
	name, value, _ := strings.Cut(arg, "=")
	if value == "" {
		return arg
	}
	return name + "=" + traceWord(value)
}

func traceWord(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \t\n'\"\\$`|&;<>()*?[") {
		return arg
	}
	return singleQuoteForReuse(arg)
}
