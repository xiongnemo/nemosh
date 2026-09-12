package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// df, dd and stty.
//
// df's numbers come from the machine, so what is checked is the shape of the report and the
// arithmetic that turns bytes into a row -- the percentage rule especially, which is of used
// plus available rather than of the raw size. dd is checked by copying, which is the whole
// of what it does.

func TestDdCopies(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := filepath.Join(dir, "src.bin")
	if err := os.WriteFile(source, []byte("abcdefghij"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, testcase := range []struct {
		name  string
		args  []string
		stdin string
		want  string
		// counts is what the record report must say, on stderr.
		counts string
	}{
		{name: "whole file to stdout", args: []string{"if=" + source}, want: "abcdefghij", counts: "0+1 records in\n0+1 records out\n"},
		{name: "block size and count", args: []string{"if=" + source, "bs=2", "count=3"}, want: "abcdef", counts: "3+0 records in\n3+0 records out\n"},
		{name: "skip", args: []string{"if=" + source, "bs=1", "skip=3", "count=4"}, want: "defg", counts: "4+0 records in\n4+0 records out\n"},
		{
			// A short read is a whole record, which is why this is 1+1 and not 2+0.
			name: "a partial record is counted separately", args: []string{"bs=4"}, stdin: "hello",
			want: "hello", counts: "1+1 records in\n1+1 records out\n",
		},
		{name: "conv=ucase", args: []string{"conv=ucase", "status=none"}, stdin: "abc", want: "ABC"},
		{name: "conv=lcase", args: []string{"conv=lcase", "status=none"}, stdin: "ABC", want: "abc"},
		{name: "conv=swab", args: []string{"conv=swab", "status=none"}, stdin: "abcd", want: "badc"},
		{name: "conv=swab leaves an odd byte", args: []string{"conv=swab", "status=none"}, stdin: "abc", want: "bac"},
		{name: "conv=sync pads", args: []string{"bs=4", "conv=sync", "status=none"}, stdin: "ab", want: "ab\x00\x00"},
		{name: "status=none says nothing", args: []string{"if=" + source, "status=none"}, want: "abcdefghij", counts: ""},
		{name: "a product size", args: []string{"if=" + source, "bs=1x2", "count=2", "status=none"}, want: "abcd"},
		{name: "the b suffix is 512", args: []string{"if=" + source, "bs=1b", "count=1", "status=none"}, want: "abcdefghij"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			got, stderr, status := runApplet(t, "dd", testcase.args, testcase.stdin)
			if got != testcase.want || status != 0 {
				t.Fatalf("dd %v\n got %q status %d\nwant %q", testcase.args, got, status, testcase.want)
			}
			if testcase.counts != "" && stderr != testcase.counts {
				t.Fatalf("dd %v reported %q, want %q", testcase.args, stderr, testcase.counts)
			}
			if testcase.counts == "" && strings.Contains(stderr, "records") {
				t.Fatalf("dd %v reported %q when it should have been silent", testcase.args, stderr)
			}
		})
	}
}

func TestDdWritesFiles(t *testing.T) {
	t.Parallel()
	t.Run("seek with notrunc writes into the middle", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "f.bin")
		if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, status := runApplet(t, "dd", []string{"of=" + path, "bs=1", "seek=3", "conv=notrunc", "status=none"}, "XY"); status != 0 {
			t.Fatalf("status %d", status)
		}
		if got := readFileText(t, path); got != "012XY56789" {
			t.Fatalf("the file holds %q", got)
		}
	})

	t.Run("without notrunc the file is truncated", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "f.bin")
		if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, status := runApplet(t, "dd", []string{"of=" + path, "status=none"}, "XY"); status != 0 {
			t.Fatalf("status %d", status)
		}
		if got := readFileText(t, path); got != "XY" {
			t.Fatalf("the file holds %q", got)
		}
	})
}

func TestDdRefusals(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"cnt=3"},                // the typo busybox accepts silently
		{"-Z"},                   // not an operand at all
		{"bs=0"},                 // a block of nothing
		{"bs=x"},                 // not a number
		{"conv=zzz"},             // an unknown conversion
		{"conv=noerror"},         // refused on purpose rather than half-implemented
		{"conv=ucase,lcase"},     // both at once
		{"status=verbose"},       // not a level
		{"if=/nonexistent/nope"}, // a file that is not there
	} {
		if _, _, status := runApplet(t, "dd", args, ""); status == 0 {
			t.Fatalf("dd %v was accepted", args)
		}
	}
}

func TestDdNumbers(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		text string
		want int64
	}{
		{text: "0", want: 0}, {text: "10", want: 10},
		{text: "1c", want: 1}, {text: "1w", want: 2}, {text: "1b", want: 512},
		{text: "1K", want: 1024}, {text: "1k", want: 1024}, {text: "1M", want: 1024 * 1024},
		{text: "1KB", want: 1000}, {text: "2x512", want: 1024}, {text: "2x3x4", want: 24},
	} {
		got, err := ddNumber(testcase.text)
		if err != nil || got != testcase.want {
			t.Fatalf("ddNumber(%q) = %d, %v; want %d", testcase.text, got, err, testcase.want)
		}
	}
	for _, bad := range []string{"", "x", "1z", "-1", "1Kx"} {
		if _, err := ddNumber(bad); err == nil {
			t.Fatalf("ddNumber(%q) was accepted", bad)
		}
	}
}

func TestDfReport(t *testing.T) {
	t.Parallel()
	got, stderr, status := runApplet(t, "df", nil, "")
	if status != 0 || stderr != "" {
		t.Fatalf("df: stderr %q status %d", stderr, status)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("df printed %q, which has no rows", got)
	}
	if lines[0] != "Filesystem           1K-blocks      Used Available Use% Mounted on" {
		t.Fatalf("df's header is %q", lines[0])
	}
	for _, line := range lines[1:] {
		if !strings.Contains(line, "%") {
			t.Fatalf("df row %q has no percentage", line)
		}
	}
	human, _, status := runApplet(t, "df", []string{"-h"}, "")
	if status != 0 {
		t.Fatalf("df -h exited %d", status)
	}
	if !strings.HasPrefix(human, "Filesystem                Size      Used Available Use% Mounted on\n") {
		t.Fatalf("df -h's header is wrong: %q", strings.SplitN(human, "\n", 2)[0])
	}
	// An operand narrows it to the one filesystem that path is on.
	one, _, status := runApplet(t, "df", []string{t.TempDir()}, "")
	if status != 0 || len(strings.Split(strings.TrimRight(one, "\n"), "\n")) != 2 {
		t.Fatalf("df on one directory printed %q status %d", one, status)
	}
}

// TestDfArithmetic covers the two rules that are decisions rather than measurements.
func TestDfArithmetic(t *testing.T) {
	t.Parallel()
	for _, testcase := range []struct {
		name            string
		used, available uint64
		want            int
	}{
		// The reference rounds to nearest: used*100 + total/2, over total.
		{name: "a real volume", used: 893761600 * 1024, available: 105593520 * 1024, want: 89},
		{name: "empty", used: 0, available: 100, want: 0},
		{name: "full", used: 100, available: 0, want: 100},
		{name: "half", used: 50, available: 50, want: 50},
		{name: "rounds up at the half", used: 455, available: 545, want: 46},
		{name: "nothing at all is not a division by zero", used: 0, available: 0, want: 0},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			usage := filesystemUsage{used: testcase.used, available: testcase.available}
			if got := usage.percentUsed(); got != testcase.want {
				t.Fatalf("percentUsed() = %d, want %d", got, testcase.want)
			}
		})
	}
	for _, testcase := range []struct {
		bytes uint64
		human bool
		want  string
	}{
		// Blocks round up: part of a block still occupies the block.
		{bytes: 1, want: "1"}, {bytes: 1024, want: "1"}, {bytes: 1025, want: "2"}, {bytes: 0, want: "0"},
		// One decimal always, which is busybox's rule for df and not GNU's for du.
		{bytes: 1024 * 1024 * 1024, human: true, want: "1.0G"},
		{bytes: 152372838, human: true, want: "145.3M"},
		{bytes: 512, human: true, want: "512"},
	} {
		if got := diskAmount(testcase.bytes, testcase.human); got != testcase.want {
			t.Fatalf("diskAmount(%d, %v) = %q, want %q", testcase.bytes, testcase.human, got, testcase.want)
		}
	}
}

// TestSttyWithoutATerminal covers what stty does where a test can reach it.
//
// The settings themselves need a console, which `go test` does not give it -- so what is
// checked is that it says so rather than guessing, and that an option it does not have is
// refused rather than accepted and ignored.
func TestSttyWithoutATerminal(t *testing.T) {
	t.Parallel()
	_, stderr, status := runApplet(t, "stty", []string{"size"}, "")
	if status == 0 {
		t.Fatal("stty answered without a terminal")
	}
	if !strings.Contains(stderr, "not a terminal") {
		t.Fatalf("stty said %q", stderr)
	}
	for _, args := range [][]string{{"-icanon"}, {"min", "1"}, {"9600"}, {"raw"}} {
		if _, _, status := runApplet(t, "stty", args, ""); status == 0 {
			t.Fatalf("stty %v was accepted", args)
		}
	}
}
