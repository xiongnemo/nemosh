package applets

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// sort holds its input as one block and its lines as views into it, where it made a string of
// every line: 92 MB of lines peaked at 290 MB, where busybox's sort peaks at 188.
func TestSort_readsItsInputAsOneBlock(t *testing.T) {
	text := strings.Repeat("a line of text to be sorted\r\n", 10000) + "a last line with no end"
	allocations := testing.AllocsPerRun(3, func() {
		lines, err := readSortLines(strings.NewReader(text), false)
		if err != nil || len(lines) != 10001 {
			t.Fatalf("%d lines, %v", len(lines), err)
		}
	})
	if allocations > 60 {
		t.Errorf("10,001 lines from a pipe made %.0f allocations, want a few dozen at most", allocations)
	}
	path := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	allocations = testing.AllocsPerRun(3, func() {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		lines, err := readSortLines(file, false)
		if err != nil || len(lines) != 10001 || lines[0] != "a line of text to be sorted" || lines[10000] != "a last line with no end" {
			t.Fatalf("%d lines, %v", len(lines), err)
		}
	})
	if allocations > 20 {
		t.Errorf("10,001 lines from a file made %.0f allocations, want a handful", allocations)
	}
	// An operand is read whole by its size through the view, and its lines are not copied into a
	// second slice: appending the first file's lines to nothing made one, 46 MB of them.
	ctx := WithProcessView(context.Background(), hostProcessView{cwd: filepath.Dir(path)})
	before := testing.AllocsPerRun(3, func() {
		lines, err := readSortInputs(ctx, ProcessViewFromContext(ctx), []string{"input"}, false, nil)
		if err != nil || len(lines) != 10001 {
			t.Fatalf("%d lines, %v", len(lines), err)
		}
	})
	if before > 40 {
		t.Errorf("one operand of 10,001 lines made %.0f allocations, want a few dozen at most", before)
	}
	var grown runtime.MemStats
	runtime.ReadMemStats(&grown)
	start := grown.TotalAlloc
	if _, err := readSortInputs(ctx, ProcessViewFromContext(ctx), []string{"input"}, false, nil); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&grown)
	if allocated, want := grown.TotalAlloc-start, uint64(len(text))+16*10001+64<<10; allocated > want {
		t.Errorf("reading %d bytes of operand allocated %d bytes, want at most %d: the block, a view a line, and buffers", len(text), allocated, want)
	}
	if lines, _ := readSortLines(strings.NewReader("b\x00a\x00"), true); len(lines) != 2 || lines[0] != "b" || lines[1] != "a" {
		t.Errorf("-z read %q", lines)
	}
	if lines, _ := readSortLines(strings.NewReader(""), false); len(lines) != 0 {
		t.Errorf("nothing read as %q", lines)
	}
	if lines, _ := readSortLines(strings.NewReader("\n\n"), false); len(lines) != 2 || lines[0] != "" {
		t.Errorf("two empty lines read as %q", lines)
	}
}
