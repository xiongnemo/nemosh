package runtime

import (
	"context"
	"errors"
	"fmt"
)

func (r Runtime) applyRedirects(args []string) ([]string, Streams, func() error, error) {
	tokens := make([]shellToken, len(args))
	for index, arg := range args {
		kind := tokenWord
		if isRedirectToken(arg) {
			kind = tokenRedirect
		}
		parsed := &word{parts: []wordPart{{kind: wordPartLiteral, text: arg}}}
		tokens[index] = shellToken{kind: kind, value: arg, parsed: parsed}
	}
	command, operations, err := parseRedirects(tokens)
	if err != nil {
		return nil, Streams{}, func() error { return nil }, err
	}
	table, err := r.fds.clone()
	if err != nil {
		return nil, Streams{}, func() error { return nil }, err
	}
	if err := r.applyRedirectOperations(table, operations); err != nil {
		return nil, Streams{}, func() error { return nil }, errors.Join(err, table.closeAll())
	}
	return tokenValues(command), table.streams(), table.closeAll, nil
}

func (r Runtime) runCommandWithRedirectOperations(ctx context.Context, command []shellToken, operations []redirectOperation) int {
	args := tokenValues(command)
	r.arrayOperands = arrayLiteralOperands(command)
	return r.withAppliedRedirectsFor(len(args) > 0 && isSpecialBuiltin(args[0]), operations, func(redirected Runtime) lineResult {
		return lineResult{status: redirected.runCommand(ctx, args)}
	}).status
}

// arrayLiteralOperands is which of a declaration utility's operands were written as array
// literals, `x=(a b)`, by their text. The builtin sees only text, and a quoted `x='(a b)'` is
// the same text, where bash makes a string of it; see declareName.
func arrayLiteralOperands(command []shellToken) map[string]bool {
	var operands map[string]bool
	for _, token := range command {
		if token.arrayLiteral {
			if operands == nil {
				operands = make(map[string]bool)
			}
			operands[token.value] = true
		}
	}
	return operands
}

// reportFailedRedirection says why a redirection failed, but for a heredoc whose body did not
// expand, which said so as it failed.
func (r Runtime) reportFailedRedirection(err error) {
	if !errors.Is(err, errHeredocBody) {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
	}
}

// withAppliedRedirects runs a command with its redirections, already expanded, in force for
// its duration and no longer. The control a command answers with -- `. ./lib.sh 2>/dev/null`
// that exits -- passes through untouched.
func (r Runtime) withAppliedRedirects(operations []redirectOperation, run func(Runtime) lineResult) lineResult {
	return r.withAppliedRedirectsFor(false, operations, run)
}

// withAppliedRedirectsFor is withAppliedRedirects for a command that may be a special builtin,
// whose failed redirection ends the script, status 1, as POSIX 2.8.1 has it and busybox does
// -- under `||` too, and in an if. It was said and the script went on.
func (r Runtime) withAppliedRedirectsFor(special bool, operations []redirectOperation, run func(Runtime) lineResult) lineResult {
	table, err := r.fds.clone()
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return lineResult{status: 1}
	}
	if err := r.applyRedirectOperations(table, operations); err != nil {
		cleanupErr := table.closeAll()
		r.reportFailedRedirection(errors.Join(err, cleanupErr))
		if special {
			r.raiseShellErrorWith(1)
		}
		return lineResult{status: 1}
	}
	result := run(r.withFDTable(table))
	if err := table.closeAll(); err != nil && result.status == 0 {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		result.status = 1
	}
	return result
}
