package applets

import (
	"testing"
	"unicode/utf16"
)

// WideCharToMultiByte says whether it put in the default character through a pointer to a
// flag, and wideToMulti made that pointer a uintptr a statement before the call. A uintptr is
// no pointer to the runtime: when the call grew the goroutine's stack, the stack moved and the
// flag with it, Windows wrote where the flag had been, and wideToMulti read the flag it had,
// unset -- so `iconv -c` kept the '?' it should have passed over, now and then, and Windows wrote
// four bytes into a stack that was no longer there. Measured in the suite: 12002 bytes where
// 12000 were wanted, once. Called at every depth of a fresh goroutine's stack, the call grows
// it at one of them.
func TestWideToMulti_saysItUsedTheDefaultWhereverTheStackGrows(t *testing.T) {
	wide := utf16.Encode([]rune("中"))
	for depth := range 600 {
		used := make(chan bool)
		go func() { used <- atDepth(depth, wide) }()
		if !<-used {
			t.Fatalf("at depth %d, wideToMulti said 中 in Latin-1 was not the default character", depth)
		}
	}
}

// atDepth calls wideToMulti depth frames down, each frame holding enough to move the point the
// stack runs out at.
//
//go:noinline
func atDepth(depth int, wide []uint16) bool {
	var pad [8]uint64
	if depth > 0 {
		pad[depth%8] = uint64(depth)
		return atDepth(depth-1, wide) && pad[depth%8] == uint64(depth)
	}
	out := make([]byte, 16)
	_, usedDefault, _ := wideToMulti(28591, wcNoBestFitChars, wide, out, true)
	return usedDefault
}
