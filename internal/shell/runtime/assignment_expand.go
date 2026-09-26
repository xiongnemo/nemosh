package runtime

import (
	"context"
	"fmt"
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
	target, _, found := strings.Cut(first.text, "=")
	name, _ := splitAssignmentTarget(target)
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
// count, because `x='a]=b'` is a command name. `]+=` appends, as `A['x']+='bar'` does.
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
		if strings.Contains(part.text, "]=") || strings.Contains(part.text, "]+=") {
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

// isDeclarationUtility reports a command word that is, as written, one of the builtins whose
// `name=value` operands are assignments: expanded as one, not split and not globbed, which
// POSIX now says for export and readonly and both references do for all five. They were
// expanded as arguments, so `export PATH=$PATH:/x` with a space anywhere in PATH -- which on
// Windows is `C:/Program Files` -- cut the value at the space and exported what followed as
// a name of its own.
func isDeclarationUtility(item word) bool {
	if !isUnquotedLiteralWord(item) {
		return false
	}
	var text strings.Builder
	for _, part := range item.parts {
		text.WriteString(part.text)
	}
	return isDeclarationName(text.String())
}

// isDeclarationName reports whether a command name is one of the five declaration utilities.
func isDeclarationName(name string) bool {
	switch name {
	case "export", "readonly", "local", "declare", "typeset":
		return true
	}
	return false
}

// expandAssignmentWord expands one assignment word, unsplit, with its tildes honoured. It is
// neither brace-expanded nor globbed: `x={X,Y}` is the text {X,Y} and `foo=*` is a star in
// both references. Both went through the command word's expansion, so `x={X,Y}` was two
// assignments, x=X then x=Y, and `foo=*` beside files named foo=a and foo=b globbed the whole
// word and assigned foo=b.
//
// braces is for a declaration utility's operand, which bash does brace-expand: `export
// y={X,Y}` exports y=X and then y=Y there. busybox has no brace expansion at all. Neither
// globs one.
func (r Runtime) expandAssignmentWord(ctx context.Context, item word, braces bool, savedStatus int) []string {
	expander := r.expandingAssignment()
	if !braces {
		return expander.expandWord(ctx, assignmentTildeWord(item), savedStatus)
	}
	var values []string
	for _, braced := range expandBraceWord(item) {
		values = append(values, expander.expandWord(ctx, assignmentTildeWord(braced), savedStatus)...)
	}
	return values
}

// assignmentTildeWord marks an assignment word, whose tilde-prefixes begin after the `=` and
// after every unquoted `:` -- which is what makes `PATH=~/bin:~/sbin` work. See
// tilde_expand.go; only the leading one after the `=` was expanded.
//
// Not an array literal, whose text is its elements': they are expanded one by one when the
// literal is taken apart, and a tilde pass over the whole text half-expanded `[k]=~:~:~`.
func assignmentTildeWord(item word) word {
	if len(item.parts) == 0 {
		return item
	}
	first := item.parts[0]
	_, value, found := strings.Cut(first.text, "=")
	if first.kind != wordPartLiteral || first.quote != quoteUnquoted || !found || strings.HasPrefix(value, "(") {
		return item
	}
	marked := item
	marked.assignmentTilde = true
	return marked
}

// assignArrayElementText writes one array element from an already-expanded value.
//
// The path for `a[0]=$(cmd)`: applyArrayAssignments settles the forms whose value is
// a plain literal before expansion, and a value that had to be expanded arrives
// here instead. Both end at shellArrays.setElement, so the two spellings cannot
// drift apart.
func (r Runtime) assignArrayElementText(ctx context.Context, reference arrayReference, value string) int {
	index, err := r.resolveSubscript(ctx, reference.subscript)
	if err != nil {
		fmt.Fprintln(r.streams.Stderr, err)
		return 1
	}
	// A negative subscript counts from the end, so it needs the length the array has
	// now. This refused one outright while the literal `a[-1]=x` path counted it; the
	// two answered the same question differently, and there is one of them now.
	index, withinRange := countFromEnd(index, r.arrays.span(reference.name))
	if !withinRange {
		fmt.Fprintf(r.streams.Stderr, "%s: bad array subscript\n", reference.subscript)
		return 1
	}
	r.arrays.setElement(reference.name, index, value)
	r.syncArrayScalar(reference.name)
	return 0
}
