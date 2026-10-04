package runtime

import (
	"math"
	"slices"
	"sort"
)

// An indexed array's storage, split from array.go to stay under the 250-line ceiling.
//
// bash's indexed arrays are sparse, to the largest index an int64 holds: `a=(p); a[3]=z` has
// two elements and `${!a[@]}` is `0 3`, and `a[0x7FFFFFFFFFFFFFFF]=x` is one element more.
// An array here was a slice grown to its largest index, with a map beside it saying which
// slots were set. So `a[10000000]=x` took 160 MB, and that last assignment grew a slice
// without end. Now an array is the elements it has, by index, and nothing else.

// indexedArray is one indexed array.
type indexedArray struct {
	elements map[int]string
	// sorted is the indices in order, and nil once a change has made it stale.
	sorted []int
	// unassigned is an array declared and never given anything; see declareUnassigned.
	unassigned bool
}

func newIndexedArray() *indexedArray { return &indexedArray{elements: map[int]string{}} }

func (array *indexedArray) clone() *indexedArray {
	copied := newIndexedArray()
	for index, value := range array.elements {
		copied.elements[index] = value
	}
	copied.unassigned = array.unassigned
	return copied
}

// indices is the array's indices in order, which the caller must not change.
func (array *indexedArray) indices() []int {
	if array.sorted == nil {
		array.sorted = make([]int, 0, len(array.elements))
		for index := range array.elements {
			array.sorted = append(array.sorted, index)
		}
		sort.Ints(array.sorted)
	}
	return array.sorted
}

func (array *indexedArray) store(index int, value string) {
	if _, exists := array.elements[index]; !exists {
		array.sorted = nil
	}
	array.elements[index] = value
	array.unassigned = false
}

// has reports an indexed array of this name, empty or not.
func (a *shellArrays) has(name string) bool {
	_, ok := a.indexed[name]
	return ok
}

// valueAt is one element, and whether that index is set.
func (a *shellArrays) valueAt(name string, index int) (string, bool) {
	array, ok := a.indexed[name]
	if !ok {
		return "", false
	}
	value, set := array.elements[index]
	return value, set
}

// indexedFor is the array of this name, made empty if there was none.
func (a *shellArrays) indexedFor(name string) *indexedArray {
	if a.indexed == nil {
		a.indexed = map[string]*indexedArray{}
	}
	array, ok := a.indexed[name]
	if !ok {
		array = newIndexedArray()
		a.indexed[name] = array
	}
	return array
}

// unsetElement removes one subscript, leaving a gap, as bash does: `unset a[1]` on `(x y z)`
// leaves indices 0 and 2 holding x and z.
func (a *shellArrays) unsetElement(name string, index int) {
	if array, ok := a.indexed[name]; ok {
		if _, exists := array.elements[index]; exists {
			delete(array.elements, index)
			array.sorted = nil
		}
	}
}

// span is one past the highest index set, which is what a negative subscript counts back
// from and where `a+=(x)` begins, as bash counts.
func (a *shellArrays) span(name string) int {
	array, ok := a.indexed[name]
	if !ok {
		return 0
	}
	indices := array.indices()
	if len(indices) == 0 {
		return 0
	}
	if last := indices[len(indices)-1]; last < math.MaxInt {
		return last + 1
	}
	return math.MaxInt
}

// isLive reports whether one index of a name is set.
func (a *shellArrays) isLive(name string, index int) bool {
	_, set := a.valueAt(name, index)
	return set
}

// liveIndices is the set subscripts of a name, in order: what `${!a[@]}` answers and what
// the `[@]` and `[*]` forms are built from.
func (a *shellArrays) liveIndices(name string) []int {
	array, ok := a.indexed[name]
	if !ok {
		return nil
	}
	return slices.Clone(array.indices())
}

// liveValues is `${a[@]}`: the elements that are set, in index order.
func (a *shellArrays) liveValues(name string) []string {
	array, ok := a.indexed[name]
	if !ok {
		return nil
	}
	values := make([]string, 0, len(array.elements))
	for _, index := range array.indices() {
		values = append(values, array.elements[index])
	}
	return values
}

// indexedNames is every indexed array's name.
func (a *shellArrays) indexedNames() []string {
	names := make([]string, 0, len(a.indexed))
	for name := range a.indexed {
		names = append(names, name)
	}
	return names
}

// take is the array of this name as it stands, for `local` to put back with put.
func (a *shellArrays) take(name string) (*indexedArray, bool) {
	array, ok := a.indexed[name]
	return array, ok
}

func (a *shellArrays) put(name string, array *indexedArray) {
	if a.indexed == nil {
		a.indexed = map[string]*indexedArray{}
	}
	a.indexed[name] = array
}
