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
	return r.withAppliedRedirects(operations, func(redirected Runtime) lineResult {
		return lineResult{status: redirected.runCommand(ctx, tokenValues(command))}
	}).status
}

// withAppliedRedirects runs a command with its redirections, already expanded, in force for
// its duration and no longer. The control a command answers with -- `. ./lib.sh 2>/dev/null`
// that exits -- passes through untouched.
func (r Runtime) withAppliedRedirects(operations []redirectOperation, run func(Runtime) lineResult) lineResult {
	table, err := r.fds.clone()
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return lineResult{status: 1}
	}
	if err := r.applyRedirectOperations(table, operations); err != nil {
		cleanupErr := table.closeAll()
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", errors.Join(err, cleanupErr))
		return lineResult{status: 1}
	}
	result := run(r.withFDTable(table))
	if err := table.closeAll(); err != nil && result.status == 0 {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		result.status = 1
	}
	return result
}
