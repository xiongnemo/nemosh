package runtime

import (
	"context"
	"strconv"
	"strings"
)

// Indexed arrays: `a=(one two three)`, `${a[0]}`, `${a[@]}`, `${#a[@]}`,
// `a[1]=x`, `a+=(four)`, `${!a[@]}`.
//
// Not POSIX -- neither dash nor ash has them -- so this follows bash, measured:
//
//	a=(one "two words" three)
//	echo ${a[0]}                one
//	echo ${a[@]}                one two words three
//	echo ${#a[@]}               3
//	echo ${#a[0]}               3          -- the length of the element
//	for i in "${a[@]}"          three iterations, the middle one with its blank
//	for i in ${a[@]}            four iterations, because unquoted words split
//	echo "${a[*]}"              one two words three, joined into one word
//	echo ${!a[@]}               0 1 2
//	a+=(four); echo ${#a[@]}    4
//
// The distinction that carries the whole feature is `"${a[@]}"` against
// `"${a[*]}"`: the first is one word per element, so an element containing a
// blank survives, and the second is a single word joined by IFS. Without the
// first there is no reason to have arrays at all -- a string would do.
//
// Storage is separate from the scalar variables rather than encoded into them.
// Packing elements into one string with a separator is the implementation that
// looks cheaper and then cannot represent an element containing that separator,
// which is exactly the case arrays exist for.

// shellArrays is the array store. Separate from vars, and a pointer so the value
// receiver on Runtime can still mutate it, like every other piece of shell state
// here.
type shellArrays struct {
	values map[string][]string
	// present is which indices of each name are actually set. An indexed array in
	// bash is sparse: `a=(p); a[3]=z` has two elements, not four, so `${#a[@]}` is 2
	// and `${!a[@]}` is `0 3`. Held beside the slice rather than replacing it with a
	// map, because the slice is what keeps the elements in index order for free.
	//
	// Without this `"${a[@]}"` over a sparse array produced phantom empty fields --
	// `p`, ``, ``, `z` -- and a loop over it ran four times.
	present map[string]map[int]bool
	// associative holds the `declare -A` names. A separate map because the two kinds
	// answer different questions; see array_associative.go.
	associative map[string]*associativeArray
}

func newShellArrays() *shellArrays {
	return &shellArrays{values: map[string][]string{}, present: map[string]map[int]bool{}}
}

func (a *shellArrays) get(name string) ([]string, bool) {
	elements, ok := a.values[name]
	return elements, ok
}

func (a *shellArrays) set(name string, elements []string) {
	a.values[name] = append([]string(nil), elements...)
	delete(a.present, name)
	for index := range elements {
		a.mark(name, index)
	}
	if len(elements) == 0 {
		a.mark(name)
	}
}

func (a *shellArrays) append(name string, elements []string) {
	start := len(a.values[name])
	a.values[name] = append(a.values[name], elements...)
	for offset := range elements {
		a.mark(name, start+offset)
	}
}

// setElement writes one index, growing the array with empty strings if the index
// is past the end. bash does the same, which is what makes `a[5]=x` on a
// three-element array leave gaps rather than fail.
func (a *shellArrays) setElement(name string, index int, value string) {
	elements := a.values[name]
	for len(elements) <= index {
		elements = append(elements, "")
	}
	elements[index] = value
	a.values[name] = elements
	a.mark(name, index)
}

func (a *shellArrays) unset(name string) {
	delete(a.values, name)
	delete(a.present, name)
}

// clone is what a subshell gets: the parent's arrays are visible inside it and a
// mutation there does not escape. Measured against bash --
//
//	a=(1 2); (echo ${a[0]})   prints 1, so they are inherited
//	a=(1 2); (a[0]=9); echo ${a[0]}   prints 1, so a write stays inside
//
// The elements are copied as well as the map, because appending to an inherited
// array in a subshell would otherwise write through the shared backing slice into
// the parent. Leaving this off the snapshot entirely is what made every array
// assignment in a subshell or a pipeline stage a nil map write: `(a=(1 2))` died
// with a Go stack trace where a shell should have printed nothing at all.
func (a *shellArrays) clone() *shellArrays {
	copied := &shellArrays{
		values:  make(map[string][]string, len(a.values)),
		present: make(map[string]map[int]bool, len(a.present)),
	}
	for name, elements := range a.values {
		copied.values[name] = append([]string(nil), elements...)
	}
	for name, set := range a.present {
		copied.present[name] = make(map[int]bool, len(set))
		for index := range set {
			copied.present[name][index] = true
		}
	}
	if len(a.associative) > 0 {
		copied.associative = make(map[string]*associativeArray, len(a.associative))
		for name, array := range a.associative {
			copied.associative[name] = array.clone()
		}
	}
	return copied
}

// arrayReference is a parsed `name[subscript]`.
type arrayReference struct {
	name string
	// subscript is `@`, `*`, or a decimal index.
	subscript string
}

// parseArrayReference reads `name[subscript]`, reporting whether the text is one.
func parseArrayReference(text string) (arrayReference, bool) {
	open := strings.IndexByte(text, '[')
	if open <= 0 || !strings.HasSuffix(text, "]") {
		return arrayReference{}, false
	}
	name := text[:open]
	subscript := text[open+1 : len(text)-1]
	if !isValidVariableName(name) || subscript == "" {
		return arrayReference{}, false
	}
	return arrayReference{name: name, subscript: subscript}, true
}

// elementsFor resolves a reference to the fields it produces, and reports whether
// the name is an array at all.
//
// A scalar answers to `[0]` and to `[@]`, which is bash's rule: every variable is
// an array of one as far as a subscript is concerned. That is what keeps
// `${x[0]}` from being an error for an ordinary variable.
func (r Runtime) elementsFor(ctx context.Context, reference arrayReference) ([]string, bool) {
	// An associative name is answered by key rather than by index, and `[@]` gives
	// its values in the order its keys come out in.
	if r.arrays.isAssociative(reference.name) {
		switch reference.subscript {
		case "@", "*":
			return r.arrays.valuesOf(reference.name), true
		}
		value, present := r.arrays.lookupKey(reference.name, r.resolveKey(ctx, reference.subscript))
		if !present {
			return nil, true
		}
		return []string{value}, true
	}
	elements, isArray := r.arrays.get(reference.name)
	if !isArray {
		value, exists := r.vars[reference.name]
		if !exists {
			return nil, false
		}
		elements = []string{value}
	}
	switch reference.subscript {
	case "@", "*":
		if isArray {
			return r.arrays.liveValues(reference.name), true
		}
		return elements, true
	}
	// A subscript is an expression, not a literal: `${a[$i]}` and `${a[1+1]}` both
	// have to resolve. See array_subscript.go for what this used to do instead.
	index, ok := r.subscriptIndex(ctx, reference.subscript, len(elements))
	if !ok || index >= len(elements) {
		// Out of range is the empty string, not an error: a script testing
		// `${a[9]}` for emptiness is asking a reasonable question.
		return nil, true
	}
	return []string{elements[index]}, true
}

// isValidVariableName is the name grammar POSIX gives: a letter or underscore,
// then letters, digits and underscores.
func isValidVariableName(name string) bool {
	if name == "" {
		return false
	}
	for index, r := range name {
		switch {
		case r == '_', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case index > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// arrayIndices is `${!a[@]}`: the subscripts that exist, as words.
func (r Runtime) arrayIndices(name string) []string {
	if r.arrays.isAssociative(name) {
		return r.arrays.keysOf(name)
	}
	elements, ok := r.arrays.get(name)
	if !ok {
		if _, exists := r.vars[name]; exists {
			return []string{"0"}
		}
		return nil
	}
	_ = elements
	live := r.arrays.liveIndices(name)
	indices := make([]string, 0, len(live))
	for _, index := range live {
		indices = append(indices, strconv.Itoa(index))
	}
	return indices
}

// looksLikeArrayAssignment reports whether the word so far is `name=` or
// `name+=`, which is what makes the next `(` part of the word rather than a
// subshell.
func looksLikeArrayAssignment(sofar string) bool {
	name, found := strings.CutSuffix(sofar, "=")
	if !found {
		return false
	}
	name = strings.TrimSuffix(name, "+")
	return isValidVariableName(name) || isArrayElementTarget(name)
}

// isArrayElementTarget reports whether the text is `name[subscript]`, which is
// the left-hand side of `a[1]=x`.
func isArrayElementTarget(text string) bool {
	_, ok := parseArrayReference(text)
	return ok
}

// matchingParenthesis finds the `)` that closes the `(` at open, counting nesting
// so `a=(one (two))` -- which bash rejects, but which must not run off the end
// here -- terminates.
func matchingParenthesis(line string, open int) (int, bool) {
	depth := 0
	inSingle, inDouble := false, false
	for index := open; index < len(line); index++ {
		switch line[index] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '(':
			if !inSingle && !inDouble {
				depth++
			}
		case ')':
			if !inSingle && !inDouble {
				depth--
				if depth == 0 {
					return index, true
				}
			}
		}
	}
	return 0, false
}

// arrayAssignmentSpan reports the extent of an array assignment's parentheses
// when the `(` at open begins one.
//
// `sofar` is the logical line built so far, whose tail is what decides: only a
// `(` directly after `name=` or `name+=` is an assignment. The last word of that
// tail is taken, because everything before it is other commands.
func arrayAssignmentSpan(line string, open int, sofar string) (int, bool) {
	tail := sofar
	if cut := strings.LastIndexAny(tail, " \t;&|(){}\n"); cut >= 0 {
		tail = tail[cut+1:]
	}
	if !looksLikeArrayAssignment(tail) {
		return 0, false
	}
	return matchingParenthesis(line, open)
}

// isArrayAtReference reports whether the text is one of the forms that produces
// *several* fields rather than a string: `${name[@]}` and `${!name[@]}`.
//
// `${#name[@]}` is deliberately not one of them -- it is a count, and a count is
// one word. Leaving `${!a[@]}` out of this list made it expand to `0` instead of
// `0 1 2`, because the caller took only the first value.
func isArrayAtReference(text string) bool {
	body, ok := strings.CutPrefix(text, "${")
	if !ok {
		return false
	}
	body, ok = strings.CutSuffix(body, "}")
	if !ok {
		return false
	}
	body = strings.TrimPrefix(body, "!")
	reference, ok := parseArrayReference(body)
	return ok && reference.subscript == "@"
}

// expandArrayParameter answers the array forms of `${...}`, reporting whether the
// body was one.
//
//	${a[0]}    one element
//	${a[@]}    every element, one field each
//	${a[*]}    every element, joined -- so a single field the caller may split
//	${#a[@]}   how many
//	${#a[0]}   the length of that element
//	${!a[@]}   the subscripts
func (r Runtime) expandArrayParameter(ctx context.Context, body string) ([]string, bool) {
	if count, ok := strings.CutPrefix(body, "#"); ok {
		reference, ok := parseArrayReference(count)
		if !ok {
			return nil, false
		}
		elements, exists := r.elementsFor(ctx, reference)
		if reference.subscript == "@" || reference.subscript == "*" {
			return []string{strconv.Itoa(len(elements))}, true
		}
		if !exists || len(elements) == 0 {
			return []string{"0"}, true
		}
		return []string{strconv.Itoa(len([]rune(elements[0])))}, true
	}
	if indices, ok := strings.CutPrefix(body, "!"); ok {
		reference, ok := parseArrayReference(indices)
		if !ok || (reference.subscript != "@" && reference.subscript != "*") {
			return nil, false
		}
		return r.arrayIndices(reference.name), true
	}
	reference, ok := parseArrayReference(body)
	if !ok {
		return nil, false
	}
	elements, _ := r.elementsFor(ctx, reference)
	if reference.subscript == "*" {
		// Joined into one field. Unquoted, the caller splits it on IFS again --
		// which is bash's behaviour and the reason `"${a[*]}"` is the form that
		// yields a single word.
		return []string{strings.Join(elements, " ")}, true
	}
	if len(elements) == 0 {
		return []string{""}, true
	}
	return elements, true
}
