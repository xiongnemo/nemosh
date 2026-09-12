package runtime

import "sort"

// The bookkeeping that makes an indexed array sparse, split from array.go to stay under
// the 250-line ceiling. See the `present` field there for why it exists.

// mark records that an index is set.
func (a *shellArrays) mark(name string, indices ...int) {
	if a.present == nil {
		a.present = map[string]map[int]bool{}
	}
	if a.present[name] == nil {
		a.present[name] = map[int]bool{}
	}
	for _, index := range indices {
		a.present[name][index] = true
	}
}

// unsetElement removes one subscript, leaving a gap.
//
// A gap rather than a compaction, which is bash's behaviour and the only safe one:
// `unset a[1]` on `(x y z)` leaves indices 0 and 2 holding `x` and `z`, so a later
// `${a[2]}` is still `z`. Compacting would silently shift every later index and quietly
// change what every subsequent read means.
//
// The value stays in the slice and only the mark is dropped, because the slice is
// positional -- removing from it *is* the compaction this avoids. Every reader already
// goes through liveIndices, so an unmarked slot is invisible to all of them.
func (a *shellArrays) unsetElement(name string, index int) {
	if a.present == nil {
		a.present = map[string]map[int]bool{}
	}
	if a.present[name] == nil {
		// A name whose slice was set directly, before any marking: every position
		// counts, so the set has to be filled in before one can be taken out of it.
		// Without this the delete would be a no-op against a nil map and the element
		// would stay -- which is the silent nothing this whole change is about.
		a.present[name] = map[int]bool{}
		for position := range a.values[name] {
			a.present[name][position] = true
		}
	}
	delete(a.present[name], index)
}

// liveIndices is the set subscripts of a name, in order. This is what `${!a[@]}`
// answers and what the `[@]` and `[*]` forms are built from.
func (a *shellArrays) liveIndices(name string) []int {
	set := a.present[name]
	if set == nil {
		// A name written before this bookkeeping existed, or one whose slice was set
		// directly: every position counts, which is what dense meant.
		indices := make([]int, 0, len(a.values[name]))
		for index := range a.values[name] {
			indices = append(indices, index)
		}
		return indices
	}
	indices := make([]int, 0, len(set))
	for index := range set {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	return indices
}

// liveValues is `${a[@]}`: the elements that are set, in index order, with no
// phantom fields for the gaps.
func (a *shellArrays) liveValues(name string) []string {
	elements := a.values[name]
	indices := a.liveIndices(name)
	values := make([]string, 0, len(indices))
	for _, index := range indices {
		if index < len(elements) {
			values = append(values, elements[index])
		}
	}
	return values
}
