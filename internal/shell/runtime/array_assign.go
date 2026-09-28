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
	target, value, found := cutAssignment(text)
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
	// The value's tilde-prefixes begin at its start and after each `:`, as an assignment's
	// do: `a[0]=~/x` and `a[0]=x:~/y` kept the tilde, where bash gives HOME.
	assignment.value = word{parts: []wordPart{{kind: wordPartLiteral, text: value}}, valueTilde: true}
	return assignment, true
}

// applyArrayAssignments performs the leading array assignments of a command and
// returns the words that remain.
//
// Only leading ones, and only when nothing else is on the line: `a=(x) echo hi`
// is not something bash supports either, because an array cannot be passed in a
// command's temporary environment.
func (r Runtime) applyArrayAssignments(ctx context.Context, command []word, savedStatus int) ([]word, bool) {
	if applied := r.applyMixedAssignments(ctx, command, savedStatus); applied {
		return nil, true
	}
	// In front of a command an array assignment is none, as bash reads it: a list is the
	// command's temporary string, `(1 2)`, left to the ordinary prefix path, and an element is
	// not a valid identifier. Both were made as arrays, and for good.
	if commandFollowsAssignments(command) {
		return r.refusePrefixElements(command)
	}
	applied := false
	for index, item := range command {
		assignment, ok := parseArrayAssignmentWord(item)
		if !ok {
			return command[index:], applied
		}
		r.assignArray(ctx, assignment, savedStatus)
		applied = true
		// A failed one abandons the command, the assignments after it too; see failAssignment.
		if r.expansion.shellError {
			return nil, applied
		}
	}
	return nil, applied
}

// commandFollowsAssignments reports a command word after a command's leading assignments.
func commandFollowsAssignments(command []word) bool {
	for _, item := range command {
		if _, ok := parseArrayAssignmentWord(item); !ok && !isAssignmentWord(item) {
			return true
		}
	}
	return false
}

// refusePrefixElements drops the element assignments in front of a command, each said to be
// no valid identifier as bash says it, and answers whether it dropped any.
func (r Runtime) refusePrefixElements(command []word) ([]word, bool) {
	kept := make([]word, 0, len(command))
	dropped := false
	for index, item := range command {
		assignment, ok := parseArrayAssignmentWord(item)
		if !ok && !isAssignmentWord(item) {
			return append(kept, command[index:]...), dropped
		}
		if ok && !assignment.list {
			fmt.Fprintf(r.streams.Stderr, "`%s[%s]': not a valid identifier\n", assignment.name, assignment.subscript)
			dropped = true
			continue
		}
		// So is one whose subscript or value is computed, as in `a[$i]=x f` and `a[0]=$v f`, and
		// `a[1 + 1]=x f`, whose subscript is quoted by now. Each was made, and for good.
		if target, element := writtenElementTarget(item); !ok && element {
			fmt.Fprintf(r.streams.Stderr, "`%s': not a valid identifier\n", target)
			dropped = true
			continue
		}
		kept = append(kept, item)
	}
	return kept, dropped
}

// writtenElementTarget is an element assignment's `name[subscript]` as written, and whether the
// word is one.
func writtenElementTarget(item word) (string, bool) {
	if !isAssignmentWord(item) {
		return "", false
	}
	text := printWord(item)
	name, _, found := strings.Cut(text, "[")
	if !found || !isValidVariableName(name) {
		return "", false
	}
	end := strings.Index(text, "]=")
	if appended := strings.Index(text, "]+="); appended >= 0 && (end < 0 || appended < end) {
		end = appended
	}
	if end < 0 {
		return "", false
	}
	return text[:end+1], true
}

func (r Runtime) assignArray(ctx context.Context, assignment arrayAssignment, savedStatus int) {
	r = r.assigningPlainly()
	target := assignment.name + "[" + assignment.subscript + "]"
	if assignment.list {
		// A list is an array's and not an element's: bash refuses `a[0]=(3 4)` and abandons the
		// command, where the list was written over the whole of a.
		if assignment.subscript != "" {
			fmt.Fprintf(r.streams.Stderr, "%s: cannot assign list to array member\n", target)
			r.failAssignment()
			return
		}
		r.assignCompound(ctx, assignment.name, assignment.raw, assignment.append, savedStatus)
		return
	}
	// One element, through assignVar like any other write: readonly refused, attributes
	// applied, and the element reached by assignElementByKind. This wrote the element
	// itself and so skipped the first two -- and ignored `+=`, so `a[1]+=z` replaced the
	// element where bash appends to it.
	value := strings.Join(r.expandWord(ctx, assignment.value, savedStatus), " ")
	if assignment.append {
		value = r.appendedValue(target, value)
	}
	r.assignVar(target, value)
}

// syncArrayScalar keeps `$a` answering with the first element, which is bash's
// rule: a bare reference to an array is its element zero. Without it `echo $a`
// after an array assignment would print nothing, and the difference between an
// array and a scalar would show up as an empty line rather than as a design.
func (r Runtime) syncArrayScalar(name string) {
	value, _ := r.arrays.valueAt(name, 0)
	r.vars[name] = value
}

// scalarVariable is a name read as a scalar, as `$name` reads it. An array read that way is
// its element 0 -- key 0, for an associative one -- and is unset when it has none, as bash
// reads it. This read the stored scalar: an associative array has none, so `$m` was empty
// whatever m held, and an indexed one's is the copy above, which `unset 'a[0]'` left behind.
func (r Runtime) scalarVariable(name string) (string, bool) {
	switch {
	case r.arrays == nil:
	case r.arrays.isAssociative(name):
		return r.arrays.lookupKey(name, "0")
	case r.arrays.has(name):
		return r.arrays.valueAt(name, 0)
	}
	value, set := r.vars[name]
	return value, set
}

// applyMixedAssignments runs a command made only of assignments, some of them arrays, in
// the order written. The array pass stopped at the first word that was not an array, so in
// `IFS=, parts=($line)` -- the way to split a line into an array -- the array came after a
// scalar and was never seen: it was expanded as an ordinary word and assigned as text. Left
// to right, because `x=1 a=($x)` has to see the x it has just set.
func (r Runtime) applyMixedAssignments(ctx context.Context, command []word, savedStatus int) bool {
	arrays := 0
	for _, item := range command {
		if _, ok := parseArrayAssignmentWord(item); ok {
			arrays++
		} else if !isAssignmentWord(item) {
			return false
		}
	}
	if arrays == 0 || arrays == len(command) {
		return false
	}
	for _, item := range command {
		if assignment, ok := parseArrayAssignmentWord(item); ok {
			if r.assignArray(ctx, assignment, savedStatus); r.expansion.shellError {
				return true
			}
			continue
		}
		fields := r.expandAssignmentWord(ctx, item, false, savedStatus)
		for index := range fields {
			fields[index] = keepEmptySubscript(fields[index])
		}
		assignments, _ := leadingAssignments(fields)
		if r.assignVars(assignments) != 0 {
			return true
		}
	}
	return true
}
