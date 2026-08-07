package runtime

import (
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
func (r Runtime) traceCommand(args []string) {
	if !r.options.xtrace || len(args) == 0 {
		return
	}
	prefix := defaultTracePrefix
	if custom, ok := r.vars["PS4"]; ok {
		prefix = custom
	}
	quoted := make([]string, len(args))
	for index, arg := range args {
		quoted[index] = traceWord(arg)
	}
	fmt.Fprintf(r.streams.Stderr, "%s%s\n", prefix, strings.Join(quoted, " "))
}

func traceWord(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \t\n'\"\\$`|&;<>()*?[") {
		return arg
	}
	return singleQuoteForReuse(arg)
}
