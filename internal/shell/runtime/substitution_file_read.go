package runtime

import (
	"context"
	"fmt"
	"io"
)

// `$(< file)`, bash's shorthand for `$(cat file)`: a substitution whose whole script is one
// input redirection is the file's contents. It was the empty output of a command with no
// words, which is what busybox-w32 gives and what POSIX leaves it; bash decides the
// extensions busybox lacks. bash's own test is the one used here -- one simple command, no
// words, and a single redirection, of standard input from a file -- so `$(< f 2>/dev/null)`
// is an ordinary command there as here.

// fileReadSubstitution reports the redirection when the script is exactly `< file`.
func fileReadSubstitution(script Script) (redirectOperation, bool) {
	if len(script.program) != 1 {
		return redirectOperation{}, false
	}
	listed, ok := script.program[0].(listNode)
	if !ok || len(listed.value.items) != 1 || listed.value.items[0].background {
		return redirectOperation{}, false
	}
	pipelines := listed.value.items[0].value.pipelines
	if len(pipelines) != 1 || pipelines[0].negated || len(pipelines[0].commands) != 1 {
		return redirectOperation{}, false
	}
	command, ok := pipelines[0].commands[0].(simpleCommand)
	if !ok || len(command.words) != 0 || len(command.redirects) != 1 {
		return redirectOperation{}, false
	}
	redirect := command.redirects[0]
	if redirect.kind != redirectInput || redirect.target != 0 {
		return redirectOperation{}, false
	}
	return redirect, true
}

// readRedirectedFile copies the file a `$(< file)` names to the substitution's output, and
// answers the status: 1, reported, when the file cannot be opened or read, as both
// references report it.
func (r Runtime) readRedirectedFile(ctx context.Context, redirect redirectOperation, savedStatus int) int {
	operations, ok := r.expandRedirectOperations(ctx, []redirectOperation{redirect}, savedStatus)
	if !ok || r.shellErrorRaised() {
		return 1
	}
	return r.withAppliedRedirects(operations, func(redirected Runtime) lineResult {
		if _, err := io.Copy(redirected.streams.Stdout, redirected.streams.Stdin); err != nil {
			fmt.Fprintf(redirected.streams.Stderr, "nemosh: %v\n", err)
			return lineResult{status: 1}
		}
		return lineResult{}
	}).status
}
