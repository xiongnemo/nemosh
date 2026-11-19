package applets

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"
)

// An array's key holds only its own bytes. `c[$1]++` keyed each element by a substring of the line
// it came from, and Go's map stores again the key it is handed at every assignment to it, so each
// key held the whole of two lines -- the first it was set from, through the order, and the last
// -- and counting 88 MB of Go source by first field peaked at 130 MiB, where busybox's, which
// copies a key, peaked at 38.5. Here 1,000 keys of four bytes, each set from two lines of 64 KB.
func TestAwkArray_keyHoldsOnlyItsOwnBytes(t *testing.T) {
	// Given
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	array := newAwkArray()

	// When
	for round := range 2 {
		for index := range 1000 {
			line := fmt.Sprintf("%04d", index) + strings.Repeat("x", 64<<10)
			array.set(line[:4], awkNum(float64(round)))
		}
	}
	runtime.GC()
	runtime.ReadMemStats(&after)

	// Then
	if retained := int64(after.HeapAlloc) - int64(before.HeapAlloc); retained > 1<<20 {
		t.Fatalf("1,000 keys of four bytes retained %d bytes, want at most 1 MiB", retained)
	}
	if value, present := array.get("0999"); !present || value.number != 1 || array.length() != 1000 {
		t.Fatalf("array holds %d elements and 0999 = %v, %v; want 1,000 and the second round's", array.length(), value, present)
	}
	runtime.KeepAlive(array)
}

// A removal is not a walk of the order: emptying an array of 327,666 elements in a
// `for (k in c) delete c[k]` had not finished after 90 s, where busybox's took 3. Here 100,000,
// emptied as the loop walks them, and what is left and its order after half are gone.
func TestAwkArray_removesWithoutWalkingTheOrder(t *testing.T) {
	// Given
	array := newAwkArray()
	for index := range 100000 {
		array.set(fmt.Sprintf("k%d", index), awkNum(float64(index)))
	}

	// When
	done := make(chan []string, 1)
	go func() {
		for index, key := range array.keys() {
			array.remove(key)
			if index == 49999 {
				done <- array.keys()
			}
		}
	}()

	// Then
	select {
	case left := <-done:
		if len(left) != 50000 || left[0] != "k50000" || left[49999] != "k99999" {
			t.Fatalf("after half were removed, %d are left, from %q", len(left), left[:min(2, len(left))])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("removing half of 100,000 elements took more than 5 s")
	}
}

// An awk value is no larger than its fields, rounded to its alignment: 32 bytes where a word is
// 64 bits and 20 on windows/386, where the order its fields were declared in padded it to 40 and
// to 24. An array holds one an element.
func TestAwkValue_isNoLargerThanItsFields(t *testing.T) {
	align := unsafe.Alignof(awkValue{})
	fields := unsafe.Sizeof(float64(0)) + unsafe.Sizeof("") + unsafe.Sizeof(awkValueKind(0)) + unsafe.Sizeof(false)
	if size, want := unsafe.Sizeof(awkValue{}), (fields+align-1)/align*align; size != want {
		t.Fatalf("an awkValue is %d bytes, want %d", size, want)
	}
}
