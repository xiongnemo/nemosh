package runtime

import "sort"

// Associative arrays: `declare -A m`, `m[key]=value`, `${m[key]}`, `${!m[@]}`.
//
// A different data structure from the indexed kind rather than the same one with
// string keys, because the two answer different questions. An indexed array has an
// order and gaps -- `a[5]=x` on a three-element array leaves two empty slots, which
// is bash's behaviour and something a map cannot express. An associative array has
// keys and no order of its own.
//
// The keys come out in bash's order, busybox having no associative arrays: its hash
// table's, which is the same on every run and so can be kept to. See bashHashTable. They
// came out in the order they were first set, on the belief that bash's changed from one
// run to the next, and every `declare -p` and `${!m[@]}` of more than one key disagreed.

// associativeArray is one `declare -A` name.
type associativeArray struct {
	entries map[string]string
	// table is where the keys sit, and so the order they come out in. A key overwritten
	// keeps its place, as in bash.
	table bashHashTable
	// unassigned is an array declared and never given anything; see declareUnassigned.
	unassigned bool
}

func newAssociativeArray() *associativeArray {
	return &associativeArray{entries: map[string]string{}}
}

func (a *associativeArray) set(key, value string) {
	if _, existing := a.entries[key]; !existing {
		a.table.insert(key)
	}
	a.entries[key] = value
	a.unassigned = false
}

// remove takes a key out, keeping the order of the ones that remain.
func (a *associativeArray) remove(key string) {
	if _, present := a.entries[key]; !present {
		return
	}
	delete(a.entries, key)
	a.table.remove(key)
}

// keys is the keys in the order they come out in.
func (a *associativeArray) keys() []string {
	return a.table.order()
}

func (a *associativeArray) clone() *associativeArray {
	copied := &associativeArray{entries: make(map[string]string, len(a.entries))}
	for key, value := range a.entries {
		copied.entries[key] = value
	}
	copied.table.rebuild(a.table.size, a.keys())
	copied.unassigned = a.unassigned
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

// clearAssociative empties an associative array and keeps it declared, which is what
// `m=(...)` does to one before it writes the new keys.
func (a *shellArrays) clearAssociative(name string) {
	a.declareAssociative(name)
	a.associative[name] = newAssociativeArray()
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
	return append([]string(nil), array.keys()...)
}

// valuesOf is `${m[@]}`, in the same order the keys come out in, so a script can walk
// the two together.
func (a *shellArrays) valuesOf(name string) []string {
	array, ok := a.associative[name]
	if !ok {
		return nil
	}
	keys := array.keys()
	values := make([]string, 0, len(keys))
	for _, key := range keys {
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
