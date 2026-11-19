package applets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

type diffWriteCounter struct{ writes int }

func (c *diffWriteCounter) Write(p []byte) (int, error) {
	c.writes++
	return len(p), nil
}
