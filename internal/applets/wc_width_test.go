package applets

import (
	"io"
	"testing"
)

// zeros reads as an endless run of the bytes already in the buffer it is handed, which wc's
// are, so a test can count gigabytes without writing or holding any of them.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) { return len(p), nil }

// More than an int holds on windows/386, 2 GiB, is counted as it is: `wc -c` of a 3 GiB stream
// printed a negative number there. 3 GiB of reads, 64 KiB each, take about half a second.
func TestWc_countsPastWhatAThirtyTwoBitIntHolds(t *testing.T) {
	const size = 3 << 30
	counts, err := countLinesAndBytes(io.LimitReader(zeros{}, size))
	if err != nil || counts.bytes != size || counts.lines != 0 {
		t.Fatalf("3 GiB of NULs counted %+v, %v", counts, err)
	}
	total := counts.add(counts)
	if total.bytes != 2*size {
		t.Fatalf("two of them total %d bytes, want %d", total.bytes, int64(2*size))
	}
}
