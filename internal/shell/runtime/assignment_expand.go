package runtime

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// A leading assignment's value is expanded differently from an ordinary word, and
// not doing so was the worst defect found in this whole pass.
//
// `d=$(date)` ran `Aug`. The expansion of `$(date)` was field split, which turned
// one word into six, so `leadingAssignments` -- which inspects the *strings* after
// expansion -- saw `d=Thu` and took the remaining five for a command and its
// arguments. `x=$(cmd)` is one of the most common lines in any shell script, and
// this shell ran the second word of the output. Measured before the fix:
//
//	q=$(echo a b); echo "[$q]"    ->  b: not found, and q empty
//	bash, dash, busybox ash       ->  [a b]
//
// POSIX 2.6.5 is explicit that field splitting does not apply here, and the reason
// the bug was possible is architectural: assignments were recognised after
// expansion rather than before. Recognising them on the word, like `[[ ]]` and
// array assignments already are, is what makes the exemption expressible at all.
//
// The second, smaller thing fixed here: a tilde after `=` expands. `q=~` gave the
// character back where bash gives the home directory, because the lexer only marks
// a word for tilde expansion when the *word* starts with one, and in an assignment
// it starts after the name.

// isAssignmentWord reports whether a word is a `name=value` assignment, judged
// before expansion.
//
// The name has to be an unquoted literal, which is the test every shell applies:
// `"q"=1` is a command called `q=1`, and a variable holding `q=1` is a command
// name too. Looking at the first part is enough because a name cannot contain an
// expansion -- if the `=` is not in the leading literal, there is no name.
func isAssignmentWord(item word) bool {
	if len(item.parts) == 0 {
		return false
	}
	first := item.parts[0]
	if first.kind != wordPartLiteral || first.quote != quoteUnquoted {
		return false
	}
	name, _, found := strings.Cut(first.text, "=")
	if !found {
		// The `=` may be past a subscript that is itself an expansion: `m[$k]=v`
		// begins with the literal `m[` and the equals arrives two parts later. Left
		// unrecognised, the whole word was field split, so a key holding a blank
		// became two words and the second was run as a command.
		return isElementAssignmentWord(item)
	}
	// `a[0]=x` is an assignment too, and applyArrayAssignments has already taken
	// the ones it handles; this keeps the rest from being split.
	return isValidVariableName(name) || isArrayElementTarget(name)
}

// isElementAssignmentWord reports whether a word is `name[...]=value` whose subscript
// is not a plain literal.
//
// Judged on the literal parts alone: the name and the brackets are always written,
// only the subscript and the value can be computed. A `]=` in a *quoted* part does not
// count, because `x='a]=b'` is a command name.
func isElementAssignmentWord(item word) bool {
	first := item.parts[0]
	name, _, found := strings.Cut(first.text, "[")
	if !found || !isValidVariableName(name) {
		return false
	}
	for _, part := range item.parts[1:] {
		if part.kind != wordPartLiteral || part.quote != quoteUnquoted {
			continue
		}
		if strings.Contains(part.text, "]=") {
			return true
		}
	}
	return false
}

// expandingAssignment returns a Runtime whose expansions are not field split.
//
// On the Runtime value rather than the shared options pointer, for the same reason
// errExitSuppressed is: it applies to everything this returned Runtime expands and
// to nothing else, so it cannot leak into the command that follows the assignment.
func (r Runtime) expandingAssignment() Runtime {
	r.noFieldSplit = true
	return r
}

// expandAssignmentWord expands one leading assignment, unsplit, with a tilde after
// the `=` honoured.
func (r Runtime) expandAssignmentWord(ctx context.Context, item word, savedStatus int) []string {
	return r.expandingAssignment().expandCommandWord(ctx, assignmentTildeWord(item), savedStatus)
}

// assignmentTildeWord marks the word so that a tilde straight after the `=` is
// expanded.
//
// POSIX expands one after each unquoted `:` as well, which is what makes
// `PATH=~/bin:~/sbin` work. That is not done here: the value is expanded as one
// piece and there is no per-colon hook, so only the leading tilde is handled. The
// commoner spelling by far is `x=~/dir`, and a tilde in the middle of a value is
// left alone rather than half-expanded.
func assignmentTildeWord(item word) word {
	if item.expandTilde || len(item.parts) == 0 {
		return item
	}
	first := item.parts[0]
	if first.kind != wordPartLiteral || first.quote != quoteUnquoted {
		return item
	}
	_, value, found := strings.Cut(first.text, "=")
	if !found {
		return item
	}
	// A tilde alone after the `=`, or one before a slash. `q=~x` is a user name in
	// bash and this shell has no user database to answer it with, so it is left as
	// written rather than guessed at.
	if value != "~" && !strings.HasPrefix(value, "~/") {
		// The tilde may be the whole of the value with the rest coming from an
		// expansion, as in `q=~$suffix`. That case is `~` exactly, caught above.
		return item
	}
	marked := item
	marked.parts = append([]wordPart(nil), item.parts...)
	marked.assignmentTilde = true
	return marked
}

// assignArrayElementText writes one array element from an already-expanded value.
//
// The path for `a[0]=$(cmd)`: applyArrayAssignments settles the forms whose value is
// a plain literal before expansion, and a value that had to be expanded arrives
// here instead. Both end at shellArrays.setElement, so the two spellings cannot
// drift apart.
func (r Runtime) assignArrayElementText(reference arrayReference, value string) int {
	index, err := strconv.Atoi(reference.subscript)
	if err != nil || index < 0 {
		fmt.Fprintf(r.streams.Stderr, "%s: bad array subscript\n", reference.subscript)
		return 1
	}
	r.arrays.setElement(reference.name, index, value)
	r.syncArrayScalar(reference.name)
	return 0
}
