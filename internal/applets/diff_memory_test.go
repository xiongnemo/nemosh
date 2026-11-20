package applets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// diff holds memory in proportion to the files' lines, as busybox's does. It made a table of
// every line of one against every line of the other: two files of 116,912 and 110,076 lines,
// Go's ssa/opGen.go and rewriteAMD64.go, wanted 103 GB of it, and a diff of them had committed
// 9.4 GB when it was found eleven hours on. Here 3,000 lines a side, which that table made 72 MB.
func TestDiff_comparesLargeFilesInLinearMemory(t *testing.T) {
	var left, right strings.Builder
	left.WriteString("left begins\n")
	right.WriteString("right begins\n")
	for index := range 3000 {
		switch {
		case index%7 == 0:
			fmt.Fprintf(&left, "only on the left %d\n", index)
		case index%5 == 0:
			fmt.Fprintf(&right, "only on the right %d\n", index)
		default:
			fmt.Fprintf(&left, "a line both hold %d\n", index)
			fmt.Fprintf(&right, "a line both hold %d\n", index)
		}
	}
	left.WriteString("left ends\n")
	right.WriteString("right ends\n")
	dir := t.TempDir()
	for name, text := range map[string]string{"left": left.String(), "right": right.String()} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := WithProcessView(context.Background(), hostProcessView{cwd: dir})
	var out, errs bytes.Buffer
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := newDiffApplet().Run(ctx, []string{"left", "right"}, strings.NewReader(""), &out, &errs)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, ErrExitFalse) || !strings.HasPrefix(out.String(), "--- left\n+++ right\n@@ -1,") {
		t.Fatalf("error %v, stdout begins %.60q, stderr %q", err, out.String(), errs.String())
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 8<<20 {
		t.Errorf("diff of two 3,000-line files allocated %d bytes, want at most 8 MiB", allocated)
	}
	// And it is written a buffer at a time, as busybox's stdio writes it: a write a line, to a
	// file, made the 6.8 MB diff of those two Go files take 24 s, where busybox takes 0.5.
	var counted diffWriteCounter
	if err := newDiffApplet().Run(ctx, []string{"left", "right"}, strings.NewReader(""), &counted, &errs); !errors.Is(err, ErrExitFalse) {
		t.Fatal(err)
	}
	if lines := strings.Count(out.String(), "\n"); counted.writes > 1+out.Len()/(16<<10) {
		t.Errorf("%d lines, %d bytes, were written in %d writes", lines, out.Len(), counted.writes)
	}
}

// The candidates Hunt and McIlroy's stone makes are most of what a diff of many like lines
// holds: a file of 19 MB against its sorted copy peaked at 2,380 MiB, where busybox's peaked at
// 701, its struct cand being three ints. Here 4,000 lines of forty against the same lines in
// another order, which make hundreds of thousands of candidates: each is held in 12 bytes, and
// in blocks no candidate is copied out of to grow, so 16 a candidate and 64 a line is enough.
func TestDiff_holdsItsCandidatesInTwelveBytesEach(t *testing.T) {
	// Given
	random := rand.New(rand.NewPCG(2026, 1119))
	left := make([]string, 4000)
	for index := range left {
		left[index] = fmt.Sprintf("line %d\n", random.IntN(40))
	}
	right := slices.Clone(left)
	random.Shuffle(len(right), func(i, j int) { right[i], right[j] = right[j], right[i] })
	var request diffRequest
	var keys [2][]string
	var hashes [2][]int
	for side, lines := range [2][]string{left, right} {
		keys[side], hashes[side] = make([]string, len(lines)), make([]int, len(lines))
		for index, line := range lines {
			keys[side][index], hashes[side][index] = request.diffKey(line)
		}
	}

	// When
	allocated, candidates := measureDiffMatches(keys, hashes)

	// Then
	limit := uint64(16*candidates + 64*(len(left)+len(right)))
	if candidates < 100000 || allocated > limit {
		t.Fatalf("%d candidates took %d bytes, %.1f a candidate; want at most %d", candidates, allocated,
			float64(allocated)/float64(max(candidates, 1)), limit)
	}
}

// measureDiffMatches is what diffMatches allocates, and the candidates it makes, as the least of
// three runs. MemStats counts the whole process, so whatever a goroutine an earlier test left
// running allocates in the meantime is counted too: on an arm64 runner a single measurement came
// to 8.4 MB against a limit of 4.7, and the same shuffle seed passed here. Something else has to
// allocate during all three runs to be counted.
func measureDiffMatches(keys [2][]string, hashes [2][]int) (uint64, int) {
	least, candidates := uint64(math.MaxUint64), 0
	for range 3 {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, candidates = diffMatches(keys, hashes, false)
		runtime.ReadMemStats(&after)
		least = min(least, after.TotalAlloc-before.TotalAlloc)
	}
	return least, candidates
}

type diffWriteCounter struct{ writes int }

func (c *diffWriteCounter) Write(p []byte) (int, error) {
	c.writes++
	return len(p), nil
}
