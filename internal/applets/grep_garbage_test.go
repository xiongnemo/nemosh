package applets

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// A line grep does not print makes no garbage: it was made a string to be matched, one allocation
// a line, so `grep -c func` over 92 MB collected 36 times and held 25 MB where busybox holds 5.
//
// Not under -race. regexp takes a machine from a sync.Pool for every match, and the race detector
// has the pool drop one machine in four it is given back, so a quarter of the lines made a new one:
// 7,736 allocations on windows-latest. The architectures job runs the suite without -race, and
// counts these there.
func TestGrep_aLineNotPrintedAllocatesNothing(t *testing.T) {
	if raceDetector {
		t.Skip("-race has sync.Pool drop what regexp puts back, which this would count")
	}
	text := strings.Repeat("a line that does not hold the pattern\n", 10000)
	for _, args := range [][]string{{"-c", "zzz"}, {"zzz"}, {"-c", "-v", "line"}, {"-ci", "zzz"}} {
		allocations := testing.AllocsPerRun(3, func() {
			var out, errs bytes.Buffer
			_ = newGrepApplet().Run(context.Background(), args, strings.NewReader(text), &out, &errs)
		})
		if allocations > 400 {
			t.Errorf("grep %q over 10,000 lines made %.0f allocations, want a few hundred at most", args, allocations)
		}
	}
}
