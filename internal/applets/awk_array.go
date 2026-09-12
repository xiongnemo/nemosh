package applets

// Arrays, and the order `for (k in a)` walks them in.
//
// **Insertion order**, which POSIX leaves unspecified and the two references answer
// differently -- gawk walks in hash order and busybox in its own. A stable order is chosen
// here for the reason internal/shell/runtime/array_associative.go already gives for the
// shell's associative arrays: an answer that changes between two runs of the same script
// is not something anyone can build on, and insertion order is the only one a reader could
// predict without being told.
//
// Recorded in docs/support-matrix.md, because a program that depends on a *particular*
// order is depending on something POSIX does not promise, and it should be able to find
// out what this one does.
//
// An array is **an object with an identity** rather than a pair of maps keyed by name, and
// that is what user-defined functions need: a function takes its arrays by reference, so
// the callee's parameter and the caller's variable must be two names for one array. Keying
// the elements and the order separately by name could not express that.

// awkArray is one associative array: its elements, and the order they were first set in.
type awkArray struct {
	values map[string]awkValue
	order  []string
}

func newAwkArray() *awkArray {
	return &awkArray{values: map[string]awkValue{}}
}

// get answers one element and whether it was there.
func (a *awkArray) get(key string) (awkValue, bool) {
	value, present := a.values[key]
	return value, present
}

// set writes one element, recording a new key's place in the order.
func (a *awkArray) set(key string, value awkValue) {
	if _, present := a.values[key]; !present {
		a.order = append(a.order, key)
	}
	a.values[key] = value
}

// remove deletes one element and its place in the order.
func (a *awkArray) remove(key string) {
	if _, present := a.values[key]; !present {
		return
	}
	delete(a.values, key)
	remaining := a.order[:0]
	for _, existing := range a.order {
		if existing != key {
			remaining = append(remaining, existing)
		}
	}
	a.order = remaining
}

// clear empties the array **in place**, which is the whole point: another name may be
// looking at the same array, so `delete a` on a parameter has to empty what the caller
// passed rather than give the parameter a new one.
func (a *awkArray) clear() {
	a.values = map[string]awkValue{}
	a.order = nil
}

func (a *awkArray) length() int { return len(a.values) }

// keys is the array's keys in insertion order.
//
// A copy, because the body of a `for (k in a)` may delete from the array it is walking --
// which both references allow -- and iterating the live slice while it shrinks would skip
// entries.
func (a *awkArray) keys() []string {
	keys := make([]string, 0, len(a.order))
	for _, key := range a.order {
		if _, present := a.values[key]; present {
			keys = append(keys, key)
		}
	}
	return keys
}
