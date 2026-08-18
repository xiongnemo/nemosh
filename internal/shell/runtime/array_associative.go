package runtime

import (
	"context"
	"sort"
	"strings"
)

// Associative arrays: `declare -A m`, `m[key]=value`, `${m[key]}`, `${!m[@]}`.
//
// A different data structure from the indexed kind rather than the same one with
// string keys, because the two answer different questions. An indexed array has an
// order and gaps -- `a[5]=x` on a three-element array leaves two empty slots, which
// is bash's behaviour and something a map cannot express. An associative array has
// keys and no order at all.
//
// The keys are kept in the order they were first set, which bash does not promise:
// its `${!m[@]}` comes out in hash order. Insertion order is chosen here because an
// answer that changes between two runs of the same script is not something anyone
// can build on, and it is the only order available that a reader could predict.

// associativeArray is one `declare -A` name.
type associativeArray struct {
	entries map[string]string
	// order is the keys as they were first set. A key overwritten keeps its place,
	// so `m[a]=1; m[b]=2; m[a]=3` still lists a before b.
	order []string
}

func newAssociativeArray() *associativeArray {
	return &associativeArray{entries: map[string]string{}}
}

func (a *associativeArray) set(key, value string) {
	if _, existing := a.entries[key]; !existing {
		a.order = append(a.order, key)
	}
	a.entries[key] = value
}

func (a *associativeArray) clone() *associativeArray {
	copied := &associativeArray{
		entries: make(map[string]string, len(a.entries)),
		order:   append([]string(nil), a.order...),
	}
	for key, value := range a.entries {
		copied.entries[key] = value
	}
	return copied
}

// declareAssociative marks a name as associative, which is what `declare -A` is for.
// Declaring it twice is not an error and does not empty it, matching bash.
func (a *shellArrays) declareAssociative(name string) {
	if a.associative == nil {
		a.associative = map[string]*associativeArray{}
	}
	if _, exists := a.associative[name]; !exists {
		a.associative[name] = newAssociativeArray()
	}
}

func (a *shellArrays) isAssociative(name string) bool {
	_, ok := a.associative[name]
	return ok
}

func (a *shellArrays) setKey(name, key, value string) {
	a.declareAssociative(name)
	a.associative[name].set(key, value)
}

// keysOf is `${!m[@]}`.
func (a *shellArrays) keysOf(name string) []string {
	array, ok := a.associative[name]
	if !ok {
		return nil
	}
	return append([]string(nil), array.order...)
}

// valuesOf is `${m[@]}`, in the same order the keys come out in, so a script can walk
// the two together.
func (a *shellArrays) valuesOf(name string) []string {
	array, ok := a.associative[name]
	if !ok {
		return nil
	}
	values := make([]string, 0, len(array.order))
	for _, key := range array.order {
		values = append(values, array.entries[key])
	}
	return values
}

func (a *shellArrays) lookupKey(name, key string) (string, bool) {
	array, ok := a.associative[name]
	if !ok {
		return "", false
	}
	value, present := array.entries[key]
	return value, present
}

// associativeNames lists the declared names, sorted, for `declare -p`.
func (a *shellArrays) associativeNames() []string {
	names := make([]string, 0, len(a.associative))
	for name := range a.associative {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// resolveKey turns a subscript into an associative key.
//
// Unlike an indexed subscript this is *not* arithmetic: `m[k]` means the key `k`, and
// evaluating it would look up a variable called k and use its number. The `$` forms
// are still unwrapped, because `m[$key]` is how a key held in a variable is written --
// the same limited unwrapping the indexed side does, and with the same gap for a
// subscript needing full expansion.
func (r Runtime) resolveKey(ctx context.Context, subscript string) string {
	text := strings.TrimSpace(subscript)
	if inner, ok := unwrapSubscriptParameter(text); ok {
		value, _ := r.lookupParameter(ctx, inner, 0)
		return value
	}
	// A quoted key -- `m["with space"]` -- keeps its text and loses the quotes, which
	// is what makes a key with a blank in it writable at all.
	return strings.Trim(text, `"'`)
}
