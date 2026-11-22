package runtime

import (
	"fmt"
	"strings"
)

// The forms bash mode's parser takes where nemosh's refuses them as it reads them, each refused
// as bash refuses it when it runs; see parseBudget.bash.

// bashRefusedLoopName is a for or select loop's variable as written when it is no name, in bash
// mode: `i.j`, `-`, `1x`, and a quoted or expanded one, `"q"` and `$x`, which bash refuses too,
// though they come to a name. Empty for one that is a name. busybox's parser refuses the
// unquoted ones (checkLoopName), and nemosh's still does.
func bashRefusedLoopName(tokens []shellToken) string {
	if len(tokens) == 0 || tokens[0].kind != tokenWord {
		return ""
	}
	first := tokens[0]
	if first.parsed != nil && isUnquotedLiteralWord(*first.parsed) && isValidVariableName(first.value) {
		return ""
	}
	if first.raw != "" {
		return first.raw
	}
	return first.value
}

// bashRefusedFunctionHeader reads `name() body` whose name has an expansion or quoting in it,
// `$foo-bar() {...}`, `foo-$(echo hi)() {...}`, `a"b"() {...}`: the name as written and the body's
// text. bash takes the word for a function's name and refuses it when the definition runs.
// busybox reads it as no definition at all, and nemosh's parser refuses what follows.
func bashRefusedFunctionHeader(line string) (string, string, bool) {
	end, ok := shellWordEnd(line, 0)
	if !ok || end == 0 || line[0] == '#' {
		return "", "", false
	}
	name := line[:end]
	if !strings.ContainsAny(name, "$\"'\\`") || strings.ContainsAny(name, "<>") {
		return "", "", false
	}
	if target, _, found := strings.Cut(name, "="); found {
		if variable, _ := splitAssignmentTarget(target); isValidVariableName(variable) {
			return "", "", false
		}
	}
	rest, found := strings.CutPrefix(strings.TrimLeft(line[end:], " \t"), "(")
	if !found {
		return "", "", false
	}
	rest, found = strings.CutPrefix(strings.TrimLeft(rest, " \t"), ")")
	if rest = strings.TrimSpace(rest); !found || rest == "" {
		return "", "", false
	}
	return name, rest, true
}

// refuseIdentifier answers a loop's variable or a function's name that is no name as bash does:
// the name as written, that it is not a valid identifier, status 1, and the script goes on; in
// bash's POSIX mode it ends, 2. Measured against Git for Windows' bash 5.3.
func (r Runtime) refuseIdentifier(name string) lineResult {
	fmt.Fprintf(r.streams.Stderr, "%s`%s': not a valid identifier\n", r.diagnosticPrefix(), name)
	if r.options.posix {
		return lineResult{status: 2, control: flowAbort}
	}
	return lineResult{status: 1}
}
