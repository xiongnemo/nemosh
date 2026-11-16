package runtime

import (
	"testing"
	"unsafe"
)

// Every method call copies a Runtime, so its size is what being passed by value costs: 512 bytes
// more made the loops in loop_bench_test.go a third slower, measured on 2026-11-17, and taking it
// from 536 bytes to 272 made a loop that calls a function 18% faster. What a copy shares with the
// value it was copied from is one pointer, shellState, so a field added to Runtime itself should
// be one a copy needs to have on its own -- and if it is, raise this with the measurement.
func TestRuntime_isSmallEnoughToCopy(t *testing.T) {
	const ceiling = 288 // measured 272
	if size := unsafe.Sizeof(Runtime{}); size > ceiling {
		t.Errorf("a Runtime is %d bytes, over %d: what every copy shares belongs in shellState", size, ceiling)
	}
}
