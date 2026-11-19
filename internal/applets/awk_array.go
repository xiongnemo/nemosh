package applets

import "strings"

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
//
// **A key holds only its own bytes.** `c[$1]` hands the array a substring of its line, and a map
// keeps the key it is handed -- Go's stores it again at every assignment, "as the backing storage
// may differ" -- so each element held the whole of the line it was last set from, and of the
// first through the order: a key of four bytes set from two lines of 64 KB held both. A key is
// copied once, as it is first set, and an element set again is written where it stands rather
// than through the map. Counting 88 MB of Go source by first field keeps 34 MB after a
// collection where it kept 55, busybox's taking 38.6 in all; its peak, 96 MiB where it was 132,
// is still near three times that, as Go lets the heap grow to twice what it keeps before it
// collects, which is a setting of the whole process and the shell's as well.
//
// **A removal is not a walk of the order.** It was, and emptying an array of 327,666 elements in
// a `for (k in c) delete c[k]` had not finished after 90 s; busybox's took 3. The order is the
// places of the entries, kept in blocks that no entry is copied out of to grow, which a slice
// was, twice over at each step; an entry removed is one the index no longer names, and the
// places are compacted once half of them are of removed entries.

// awkArray is one associative array: its entries in the order they were first set, and the
// place of each key's among them.
type awkArray struct {
	index  map[string]int32
	blocks [][]awkEntry
	// places is the places taken, by entries removed since the last compaction among them.
	places int
}

type awkEntry struct {
	key   string
	value awkValue
}

// awkEntryBlock is the entries of a block, 192 KB of them; the first grows to it from 8.
const awkEntryBlock = 1 << 12

func newAwkArray() *awkArray {
	return &awkArray{index: map[string]int32{}}
}

func (a *awkArray) entry(place int32) *awkEntry {
	at := uint32(place)
	return &a.blocks[at/awkEntryBlock][at%awkEntryBlock]
}

// live is whether the entry at place is one the index names, and not one removed.
func (a *awkArray) live(place int32) bool {
	at, present := a.index[a.entry(place).key]
	return present && at == place
}

// get answers one element and whether it was there.
func (a *awkArray) get(key string) (awkValue, bool) {
	if place, present := a.index[key]; present {
		return a.entry(place).value, true
	}
	return awkValue{}, false
}

// set writes one element, giving a new key a copy of its own and the next place in the order.
func (a *awkArray) set(key string, value awkValue) {
	if place, present := a.index[key]; present {
		a.entry(place).value = value
		return
	}
	key = strings.Clone(key)
	a.index[key] = a.push(awkEntry{key: key, value: value})
}

func (a *awkArray) push(entry awkEntry) int32 {
	last := len(a.blocks) - 1
	if last < 0 || len(a.blocks[last]) == awkEntryBlock {
		capacity := awkEntryBlock
		if last < 0 {
			capacity = 8
		}
		a.blocks = append(a.blocks, make([]awkEntry, 0, capacity))
		last++
	}
	a.blocks[last] = append(a.blocks[last], entry)
	a.places++
	return int32(a.places - 1)
}

// remove deletes one element, and compacts the places once half of them are of removed ones.
func (a *awkArray) remove(key string) {
	place, present := a.index[key]
	if !present {
		return
	}
	delete(a.index, key)
	*a.entry(place) = awkEntry{}
	if len(a.index) < a.places/2 {
		old, places := a.blocks, a.places
		a.blocks, a.places = nil, 0
		for at := range places {
			entry := &old[at/awkEntryBlock][at%awkEntryBlock]
			if kept, present := a.index[entry.key]; present && kept == int32(at) {
				a.index[entry.key] = a.push(*entry)
			}
		}
	}
}

// clear empties the array **in place**, which is the whole point: another name may be
// looking at the same array, so `delete a` on a parameter has to empty what the caller
// passed rather than give the parameter a new one.
func (a *awkArray) clear() {
	a.index = map[string]int32{}
	a.blocks, a.places = nil, 0
}

func (a *awkArray) length() int { return len(a.index) }

// keys is the array's keys in insertion order.
//
// A copy, because the body of a `for (k in a)` may delete from the array it is walking --
// which both references allow -- and iterating the live slice while it shrinks would skip
// entries.
func (a *awkArray) keys() []string {
	keys := make([]string, 0, len(a.index))
	for place := range int32(a.places) {
		if a.live(place) {
			keys = append(keys, a.entry(place).key)
		}
	}
	return keys
}
