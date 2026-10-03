package runtime

import (
	"context"
	"fmt"
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
	// indexed holds each indexed array as the elements it has, by index; see
	// array_sparse.go. Sparse, as bash's are: `a=(p); a[3]=z` has two elements, not four,
	// so `${#a[@]}` is 2 and `${!a[@]}` is `0 3`.
	indexed map[string]*indexedArray
	// associative holds the `declare -A` names. A separate map because the two kinds
	// answer different questions; see array_associative.go.
	associative map[string]*associativeArray
}

func newShellArrays() *shellArrays {
	return &shellArrays{indexed: map[string]*indexedArray{}}
}

// set makes the array these elements, at indices 0 onwards.
func (a *shellArrays) set(name string, elements []string) {
	array := newIndexedArray()
	for index, value := range elements {
		array.elements[index] = value
	}
	a.put(name, array)
}

// append adds elements after the highest index set, which is bash's `a+=(...)`.
func (a *shellArrays) append(name string, elements []string) {
	start := a.span(name)
	array := a.indexedFor(name)
	for offset, value := range elements {
		array.store(start+offset, value)
	}
}

// setElement writes one index. `a[5]=x` on a three-element array leaves a gap, as in bash.
func (a *shellArrays) setElement(name string, index int, value string) {
	a.indexedFor(name).store(index, value)
}

// unset removes a whole array of either kind.
//
// The associative map is cleared too, which it was not: `declare -A m; m[k]=v; unset m`
// left the name associative and its keys intact, so a later `m[j]=w` added to the old map
// instead of making a fresh indexed array. bash drops the declaration with the value, and
// `${!m[@]}` there answers `0` afterwards where this answered `k j`.
//
// The builtin never reached here at all before -- it deleted from the scalar table only --
// so `unset a` on an indexed array left that behind as well.
func (a *shellArrays) unset(name string) {
	delete(a.indexed, name)
	delete(a.associative, name)
}

// clone is what a subshell gets: the parent's arrays are visible inside it and a
// mutation there does not escape. Measured against bash --
//
//	a=(1 2); (echo ${a[0]})   prints 1, so they are inherited
//	a=(1 2); (a[0]=9); echo ${a[0]}   prints 1, so a write stays inside
//
// The elements are copied as well as the map, because a write to an inherited array in
// a subshell would otherwise reach the parent's. Leaving this off the snapshot entirely
// is what made every array assignment in a subshell or a pipeline stage a nil map write:
// `(a=(1 2))` died with a Go stack trace where a shell should have printed nothing at all.
func (a *shellArrays) clone() *shellArrays {
	copied := &shellArrays{indexed: make(map[string]*indexedArray, len(a.indexed))}
	for name, array := range a.indexed {
		copied.indexed[name] = array.clone()
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
	isArray := r.arrays.has(reference.name)
	var elements []string
	// span is what a negative subscript counts back from: nothing, for a scalar, which is its
	// element 0 alone -- `${s[-1]}` is a bad subscript in bash, and was s.
	span := 0
	if !isArray {
		value, exists := r.vars[reference.name]
		switch stack, isStack := r.callStackArray(reference.name); {
		case exists:
			elements = []string{value}
		case isStack:
			// FUNCNAME, BASH_SOURCE, BASH_LINENO: computed, behind anything the script set.
			elements, span = stack, len(stack)
		default:
			return nil, false
		}
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
	if isArray {
		span = r.arrays.span(reference.name)
	}
	index, err := r.resolveSubscript(ctx, reference.subscript)
	if err != nil {
		return nil, true
	}
	// Out of range is the empty string, not an error: a script testing `${a[9]}` for
	// emptiness is asking a reasonable question. So is an index that was unset. One before
	// the front is said as well, as bash says it -- `${a[-9]}` of three elements is empty
	// and `a: bad array subscript` -- where it was quiet.
	index, within := countFromEnd(index, span)
	switch {
	case !within:
		fmt.Fprintf(r.streams.Stderr, "%s%s: bad array subscript\n", r.diagnosticPrefix(), reference.name)
		return nil, true
	case isArray:
		if value, set := r.arrays.valueAt(reference.name, index); set {
			return []string{value}, true
		}
		return nil, true
	case index >= len(elements):
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
	if !r.arrays.has(name) {
		if _, exists := r.vars[name]; exists {
			return []string{"0"}
		}
		return nil
	}
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
// here -- terminates. An escaped character and a `$'...'` are data, as in bash, so
// `a=(x\) $'it\'s')` is two elements.
func matchingParenthesis(line string, open int) (int, bool) {
	depth := 0
	inSingle, inDouble := false, false
	for index := open; index < len(line); index++ {
		if end := ansiQuoteClose(line, index); !inSingle && !inDouble && end >= 0 {
			index = end
			continue
		}
		switch line[index] {
		case '\\':
			if !inSingle {
				index++
			}
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
