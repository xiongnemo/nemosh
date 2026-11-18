package applets

import (
	"strings"
	"testing"
	"time"
)

// The size column is busybox's smart_ulltoa5 (libbb/human_readable.c): five characters, the
// number itself up to 99999, and above it tenths of a power of 1024 under 100, whole ones over.
func TestWgetProgress_sizesAsBusyboxDoes(t *testing.T) {
	for size, want := range map[uint64]string{
		0: "    0", 500: "  500", 12345: "12345", 99999: "99999",
		100000: "97.6k", 1 << 20: "1024k", 10 << 20: "10.0M", 1 << 30: "1024M", 5 << 40: "5120G",
	} {
		if got := wgetSize5(size); got != want {
			t.Errorf("%d is %q, want %q", size, got, want)
		}
	}
}

// The meter is busybox's bb_progress_update: nothing until a second has gone by, unless the
// download is done; then the name in twenty columns, the percentage, a bar as wide as the
// console leaves after 48 columns, the size and the time left; `--:--:-- ETA` when the size is
// not known, `- stalled -` after five seconds with nothing; and a newline once it is done.
func TestWgetProgress_drawsAsBusyboxDoes(t *testing.T) {
	now := time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	var out strings.Builder
	meter := newWgetProgress(&out, "file.bin", 1000, 80, clock)
	meter.Write(make([]byte, 100))
	if out.Len() != 0 {
		t.Fatalf("drew %q within the first second", out.String())
	}
	now = now.Add(time.Second)
	meter.Write(make([]byte, 400))
	bar := func(stars int) string { return "|" + strings.Repeat("*", stars) + strings.Repeat(" ", 32-stars) + "| " }
	half := "\rfile.bin             " + " 50% " + bar(16) + "  500" + "  0:00:01 ETA"
	if out.String() != half {
		t.Fatalf("half way drew\n%q\nwant\n%q", out.String(), half)
	}
	now = now.Add(time.Second)
	meter.Write(make([]byte, 500))
	meter.finish()
	// Drawn by the last write and again by the end, as busybox's last bump and PROGRESS_END do.
	whole := "\rfile.bin             " + "100% " + bar(32) + " 1000" + "  0:00:00 ETA"
	done := half + whole + whole + "\n"
	if out.String() != done {
		t.Fatalf("at the end drew\n%q\nwant\n%q", out.String(), done)
	}

	out.Reset()
	unknown := newWgetProgress(&out, "a-name-longer-than-twenty.iso", 0, 80, clock)
	now = now.Add(time.Second)
	unknown.Write(make([]byte, 2048))
	if want := "\ra-name-longer-than-t  2048 --:--:-- ETA"; out.String() != want {
		t.Errorf("an unknown size drew %q, want %q", out.String(), want)
	}
	out.Reset()
	now = now.Add(6 * time.Second)
	unknown.Write(nil)
	if want := "\ra-name-longer-than-t  2048  - stalled -"; out.String() != want {
		t.Errorf("a stall drew %q, want %q", out.String(), want)
	}

	out.Reset()
	narrow := newWgetProgress(&out, "x", 10, 50, clock)
	narrow.Write(make([]byte, 10))
	narrow.finish()
	line := "\rx" + strings.Repeat(" ", 20) + "100%    10  0:00:00 ETA"
	if want := line + line + "\n"; out.String() != want {
		t.Errorf("a console too narrow for a bar drew %q, want %q", out.String(), want)
	}
}
