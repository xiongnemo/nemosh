package runtime

import (
	"context"
)

// A simple command's words expand in POSIX 2.9.1's order: the command name and its arguments
// first, then the assignments in front of them, one at a time, each bound while the ones after
// it expand. Its redirections are made between the two; see runSimpleWords.
//
// They were expanded together and in the order written, and every assignment was made after
// all of them had expanded, so no assignment saw another: `a=1 b=$a` left b empty, `x=6 y=$x`
// gave y the old x, and `p=1 q=$p cmd` handed cmd an empty q. busybox-w32 and bash give the
// value the assignment before it made in each case. The arguments see none of them, in both
// references: `p=7 f "$p"` passes the p there was.
//
// Leading assignments are expanded unsplit. Recognised on the word rather than on its
// expansion, which is the only place the distinction still exists: see assignment_expand.go
// for what `d=$(date)` did without this.

// assignmentPrefix is how many of a command's words are the assignments in front of it.
func assignmentPrefix(command []word) int {
	prefix := 0
	for prefix < len(command) && isAssignmentWord(command[prefix]) {
		prefix++
	}
	return prefix
}

// expandCommandArguments expands the command name and its arguments, the words after the
// assignments in front of them: a declaration utility's assignment operands unsplit.
func (r Runtime) expandCommandArguments(ctx context.Context, words []word, savedStatus int) []shellToken {
	var rest []shellToken
	declaration := false
	for index, item := range words {
		var values []string
		literal := false
		if declaration && isAssignmentWord(item) {
			values = r.expandAssignmentWord(ctx, item, true, savedStatus)
			literal = assignsArrayLiteral(item)
		} else {
			declaration = declaration || index == 0 && isDeclarationUtility(item)
			values = r.expandCommandWord(ctx, item, savedStatus)
		}
		start := len(rest)
		rest = appendWordTokens(rest, values)
		for marked := start; marked < len(rest); marked++ {
			rest[marked].arrayLiteral = literal
		}
	}
	return rest
}

// expandLeadingAssignments expands the assignments in front of a command in order, binding
// each one while the rest expand, and undoes the bindings before it returns: the caller
// makes the assignments for real, or hands them to the command for its environment.
func (r Runtime) expandLeadingAssignments(ctx context.Context, assignments []word, savedStatus int) []shellToken {
	type binding struct {
		name, value string
		set         bool
	}
	var bound []binding
	defer func() {
		for index := len(bound) - 1; index >= 0; index-- {
			if bound[index].set {
				r.vars[bound[index].name] = bound[index].value
			} else {
				delete(r.vars, bound[index].name)
			}
		}
	}()
	var tokens []shellToken
	for _, item := range assignments {
		values := r.expandAssignmentWord(ctx, item, false, savedStatus)
		tokens = appendWordTokens(tokens, values)
		for _, value := range values {
			target, text, found := cutAssignment(value)
			name, appended := splitAssignmentTarget(target)
			if !found || !isValidVariableName(name) {
				continue
			}
			previous, set := r.vars[name]
			bound = append(bound, binding{name: name, value: previous, set: set})
			if appended {
				text = previous + text
			}
			r.vars[name] = text
		}
	}
	return tokens
}

// appendWordTokens adds expanded words to a token list.
func appendWordTokens(tokens []shellToken, values []string) []shellToken {
	for _, value := range values {
		tokens = append(tokens, shellToken{kind: tokenWord, value: value})
	}
	return tokens
}
