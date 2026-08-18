package runtime

import (
	"context"
	"fmt"
	"strings"
)

// Array assignment: `a=(one two three)`, `a+=(four)`, `a[1]=x`.
//
// Handled at the *word* level, before expansion, for the same reason `[[ ]]` is:
// the elements have to be split while their quoting is still visible. Measured --
// `a=(one "two words" three)` is three elements in bash. By the time a word has
// been expanded the quotes are gone and `two words` would split into two, which
// is precisely the case an array exists to handle.

// arrayAssignment is a parsed `name=(...)`, `name+=(...)` or `name[i]=value`.
type arrayAssignment struct {
	name string
	// subscript is the text between the brackets of the `a[1]=x` form, and empty
	// otherwise. Kept as text because a subscript is an expression -- `a[1+1]=q`
	// and `a[$i]=q` both have to work -- and this parse runs before anything is
	// evaluated. See array_subscript.go.
	subscript string
	// append is the `+=` form.
	append bool
	// raw is the text between the parentheses, unexpanded. Empty for the
	// element form, which carries its value in `value`.
	raw   string
	value word
	list  bool
}

// parseArrayAssignmentWord reads one word as an array assignment.
//
// The word must be a single unquoted literal: `a=(x)` is written, not computed.
// bash agrees -- a variable holding `a=(x)` is a command name, not an assignment.
func parseArrayAssignmentWord(item word) (arrayAssignment, bool) {
	text := soleLiteralText(item)
	if text == "" {
		return arrayAssignment{}, false
	}
	target, value, found := strings.Cut(text, "=")
	if !found {
		return arrayAssignment{}, false
	}
	assignment := arrayAssignment{}
	if name, isAppend := strings.CutSuffix(target, "+"); isAppend {
		assignment.name, assignment.append = name, true
	} else {
		assignment.name = target
	}
	if reference, ok := parseArrayReference(assignment.name); ok {
		// Left as text here and resolved when the assignment runs: a subscript is
		// an expression, and this parse happens before anything is evaluated. See
		// array_subscript.go.
		assignment.name, assignment.subscript = reference.name, reference.subscript
	}
	if !isValidVariableName(assignment.name) {
		return arrayAssignment{}, false
	}
	if strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") {
		assignment.raw = value[1 : len(value)-1]
		assignment.list = true
		return assignment, true
	}
	// `a[1]=x` is an array assignment even without parentheses; a plain `a=x`
	// is not, and is left to the ordinary scalar path.
	if assignment.subscript == "" {
		return arrayAssignment{}, false
	}
	assignment.value = word{parts: []wordPart{{kind: wordPartLiteral, text: value}}}
	return assignment, true
}

// applyArrayAssignments performs the leading array assignments of a command and
// returns the words that remain.
//
// Only leading ones, and only when nothing else is on the line: `a=(x) echo hi`
// is not something bash supports either, because an array cannot be passed in a
// command's temporary environment.
func (r Runtime) applyArrayAssignments(ctx context.Context, command []word, savedStatus int) ([]word, bool) {
	applied := false
	for index, item := range command {
		assignment, ok := parseArrayAssignmentWord(item)
		if !ok {
			return command[index:], applied
		}
		r.assignArray(ctx, assignment, savedStatus)
		applied = true
	}
	return nil, applied
}

func (r Runtime) assignArray(ctx context.Context, assignment arrayAssignment, savedStatus int) {
	if !assignment.list {
		values := r.expandWord(ctx, assignment.value, savedStatus)
		index, err := r.resolveSubscript(assignment.subscript)
		if err != nil {
			fmt.Fprintln(r.streams.Stderr, err)
			return
		}
		r.arrays.setElement(assignment.name, index, strings.Join(values, " "))
		r.syncArrayScalar(assignment.name)
		return
	}
	elements := r.expandArrayElements(ctx, assignment.raw, savedStatus)
	if assignment.append {
		r.arrays.append(assignment.name, elements)
	} else {
		r.arrays.set(assignment.name, elements)
	}
	r.syncArrayScalar(assignment.name)
}

// expandArrayElements lexes the text between the parentheses and expands each
// word, which is what keeps `"two words"` one element.
//
// Lexing rather than splitting on blanks: the quoting, the parameters and the
// command substitutions inside all have to work, and the lexer already knows how.
func (r Runtime) expandArrayElements(ctx context.Context, raw string, savedStatus int) []string {
	tokens, err := scanShellTokens(strings.TrimSpace(raw))
	if err != nil {
		return nil
	}
	var elements []string
	for _, token := range tokens {
		if token.kind != tokenWord || token.parsed == nil {
			continue
		}
		elements = append(elements, r.expandCommandWord(ctx, *token.parsed, savedStatus)...)
	}
	return elements
}

// syncArrayScalar keeps `$a` answering with the first element, which is bash's
// rule: a bare reference to an array is its element zero. Without it `echo $a`
// after an array assignment would print nothing, and the difference between an
// array and a scalar would show up as an empty line rather than as a design.
func (r Runtime) syncArrayScalar(name string) {
	elements, ok := r.arrays.get(name)
	if !ok || len(elements) == 0 {
		r.vars[name] = ""
		return
	}
	r.vars[name] = elements[0]
}
