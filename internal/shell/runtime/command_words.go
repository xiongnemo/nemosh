package runtime

import (
	"context"
	"strings"
)

// expandCommandWords expands a simple command's words in POSIX 2.9.1's order: the command
// name and its arguments first, then the assignments in front of them, one at a time, each
// bound while the ones after it expand.
//
// They were expanded together and in the order written, and every assignment was made after
// all of them had expanded, so no assignment saw another: `a=1 b=$a` left b empty, `x=6 y=$x`
// gave y the old x, and `p=1 q=$p cmd` handed cmd an empty q. busybox-w32 and bash give the
// value the assignment before it made in each case. The arguments see none of them, in both
// references: `p=7 f "$p"` passes the p there was.
//
// Leading assignments are expanded unsplit. Recognised on the word rather than on its
// expansion, which is the only place the distinction still exists: see assignment_expand.go
// for what `d=$(date)` did without this. The second result is how many of the tokens they
// are, so the caller can tell them from the command without looking again at the text.
func (r Runtime) expandCommandWords(ctx context.Context, command []word, savedStatus int) ([]shellToken, int) {
	prefix := 0
	for prefix < len(command) && isAssignmentWord(command[prefix]) {
		prefix++
	}
	var rest []shellToken
	declaration := false
	for index, item := range command[prefix:] {
		var values []string
		if declaration && isAssignmentWord(item) {
			values = r.expandAssignmentWord(ctx, item, true, savedStatus)
		} else {
			declaration = declaration || index == 0 && (isDeclarationUtility(item) || r.aliasDeclares(item))
			values = r.expandCommandWord(ctx, item, savedStatus)
		}
		rest = appendWordTokens(rest, values)
	}
	leading := r.expandLeadingAssignments(ctx, command[:prefix], savedStatus)
	return append(leading, rest...), len(leading)
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
			target, text, found := strings.Cut(value, "=")
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

// aliasDeclares reports whether a command word is an alias whose first word is a declaration
// utility, whose operands are then assignments all the same: with `alias e=export`, `e
// x=$words` exports the value whole in busybox-w32, where it was split and only its first
// word exported. bash expands no alias in a script.
func (r Runtime) aliasDeclares(item word) bool {
	if !r.options.expandAliases || !isUnquotedLiteralWord(item) {
		return false
	}
	value, defined := r.aliases[soleLiteralText(item)]
	if !defined {
		return false
	}
	words, err := aliasWords(value)
	return err == nil && len(words) > 0 && isDeclarationName(words[0])
}

// appendWordTokens adds expanded words to a token list.
func appendWordTokens(tokens []shellToken, values []string) []shellToken {
	for _, value := range values {
		tokens = append(tokens, shellToken{kind: tokenWord, value: value})
	}
	return tokens
}
