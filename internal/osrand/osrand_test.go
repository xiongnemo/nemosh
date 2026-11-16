package osrand

import (
	"bytes"
	"io"
	"testing"
)

// Every byte of the buffer is written, and two reads differ: a buffer left as it was, or a source
// that repeats, fails one or the other.
func TestRead_fillsTheWholeBuffer(t *testing.T) {
	for _, size := range []int{1, 16, 4096, 1 << 20} {
		first, second := make([]byte, size), make([]byte, size)
		if n, err := Read(first); n != size || err != nil {
			t.Fatalf("Read of %d: %d, %v", size, n, err)
		}
		if _, err := io.ReadFull(Reader, second); err != nil {
			t.Fatal(err)
		}
		if size >= 16 && bytes.Equal(first, second) {
			t.Errorf("two reads of %d bytes were the same", size)
		}
		// Every value of a byte turns up in a random megabyte, and each about 4096 times.
		if size == 1<<20 {
			var counts [256]int
			for _, value := range first {
				counts[value]++
			}
			for value, count := range counts {
				if count < 3000 || count > 5200 {
					t.Errorf("byte %#x came %d times in a megabyte", value, count)
				}
			}
		}
	}
	if n, err := Read(nil); n != 0 || err != nil {
		t.Errorf("Read of nothing: %d, %v", n, err)
	}
}
