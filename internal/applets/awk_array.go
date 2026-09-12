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

// touchArrayKey records a key the first time it is seen, so the order survives.
func (in *awkInterp) touchArrayKey(name, key string) {
	if in.arrayOrders == nil {
		in.arrayOrders = map[string][]string{}
	}
	if _, present := in.arrays[name][key]; present {
		return
	}
	in.arrayOrders[name] = append(in.arrayOrders[name], key)
}

// setArrayElement writes one element, keeping the order.
func (in *awkInterp) setArrayElement(name, key string, value awkValue) {
	array := in.getArray(name)
	in.touchArrayKey(name, key)
	array[key] = value
}

// deleteArrayKey removes one element and its place in the order.
func (in *awkInterp) deleteArrayKey(name, key string) {
	array, ok := in.arrays[name]
	if !ok {
		return
	}
	if _, present := array[key]; !present {
		return
	}
	delete(array, key)
	order := in.arrayOrders[name]
	remaining := order[:0]
	for _, existing := range order {
		if existing != key {
			remaining = append(remaining, existing)
		}
	}
	in.arrayOrders[name] = remaining
}

// arrayKeys is the array's keys in insertion order.
//
// A copy, because the body of a `for (k in a)` may delete from the array it is walking --
// which both references allow -- and iterating the live slice while it shrinks would skip
// entries.
func (in *awkInterp) arrayKeys(name string) []string {
	order := in.arrayOrders[name]
	keys := make([]string, 0, len(order))
	array := in.arrays[name]
	for _, key := range order {
		if _, present := array[key]; present {
			keys = append(keys, key)
		}
	}
	return keys
}
