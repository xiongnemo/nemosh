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
	table, err := r.fds.clone()
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return 1
	}
	if err := r.applyRedirectOperations(table, operations); err != nil {
		cleanupErr := table.closeAll()
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", errors.Join(err, cleanupErr))
		return 1
	}
	status := r.withFDTable(table).runCommand(ctx, tokenValues(command))
	if err := table.closeAll(); err != nil && status == 0 {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return 1
	}
	return status
}
