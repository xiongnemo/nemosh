package applets

import (
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
	"github.com/xiongnemo/nemosh/internal/textgrid"
)

// That every column is wide enough for anything it can be asked to draw.
//
// This is the test that keeps the table aligned, and it is worth more than checking any particular
// width. A column's width is a claim about its formatter -- "five cells is enough for a percentage"
// -- and the two live in different files, so the claim rots the moment either changes. Here they are
// checked against each other: the widest value each formatter can produce, put through the column
// that displays it.
//
// The alternative was noticing later that the table had stopped lining up, which is how this came
// up: TIME+ was nine cells and Idle's cumulative CPU on this machine is `264482:43.25`, twelve.

// extremeRow is a process with the largest value in every field that anything realistic can reach.
//
// Realistic is doing work here, not hedging. A pid is a DWORD and could in principle need ten
// digits, but Windows allocates them from a low pool and a seven-digit pid does not occur; sizing
// PID for the theoretical maximum would cost three cells on every row for ever. Where a value can
// exceed its column the cell is cut and marked, which the test below covers separately.
func extremeRow() topRow {
	return topRow{
		Process: proc.Process{
			PID: 9999999, PPID: 9999999, Threads: 99999, Handles: 999999,
			Priority: 24,
			// 1023 TiB of working set, which is the widest topBytes can render before it
			// runs out of suffixes.
			WorkingSet: 1023 << 40, PrivateWorkingSet: 1023 << 40, Commit: 1023 << 40,
			// Two hundred and seventy years of CPU, well past where the format switches
			// to days.
			Kernel: 270 * 365 * 24 * time.Hour,
			State:  proc.StateRunning,
		},
		Rate: proc.ProcessRate{
			CPU: 1, ReadBytesPerSecond: 1023 << 40, WriteBytesPerSecond: 1023 << 40,
		},
		MemoryShare: 1,
		Details:     proc.Details{User: "nemo"},
	}
}

func TestTopColumns_widthsHoldEveryValue(t *testing.T) {
	row := extremeRow()
	for _, column := range topColumns {
		if column.Width == 0 {
			// The command takes the rest of the line by design; it is the only column
			// whose length nobody can predict, which is why it is last.
			continue
		}
		t.Run(column.Key, func(t *testing.T) {
			// The header has to fit too. COMMIT is six characters and its column was
			// six, which is the sort of thing that is only true by accident.
			if got := textgrid.Cells(column.Header); got > column.Width {
				t.Fatalf("header %q is %d cells in a column of %d", column.Header, got, column.Width)
			}
			if got := textgrid.Cells(column.Cell(row)); got > column.Width {
				t.Fatalf("%s renders %q, %d cells in a column of %d",
					column.Key, column.Cell(row), got, column.Width)
			}
			// And the smallest value, so a column is not merely wide enough but also
			// laid out: padded to exactly its width, whichever way it aligns.
			for _, text := range []string{column.Cell(row), column.Cell(topRow{}), column.Header} {
				if got := textgrid.Cells(padTopCell(text, column.Width, column.Right)); got != column.Width {
					t.Fatalf("%s pads %q to %d cells, want exactly %d",
						column.Key, text, got, column.Width)
				}
			}
		})
	}
}

// PRI is sized from the names it can print, which is what a priority class is on this platform.
func TestTopPriority_everyClassFitsItsColumn(t *testing.T) {
	column, ok := columnByKey("pri")
	if !ok {
		t.Fatal("no pri column")
	}
	seen := map[string]bool{}
	// Every base priority a Windows process can report, not a sample of them.
	for base := 0; base <= 31; base++ {
		name := topPriority(base)
		seen[name] = true
		if got := textgrid.Cells(name); got > column.Width {
			t.Fatalf("priority %d prints %q, %d cells in a column of %d",
				base, name, got, column.Width)
		}
	}
	// And that the column is not wider than it needs to be, which is the other half of the
	// question: the longest of these names is what the width should be.
	longest := 0
	for name := range seen {
		if cells := textgrid.Cells(name); cells > longest {
			longest = cells
		}
	}
	if longest != column.Width {
		t.Fatalf("the longest priority name is %d cells and the column is %d", longest, column.Width)
	}
}

// A value too wide for its column is cut and says so.
func TestPadTopCell_marksWhatItCuts(t *testing.T) {
	tests := []struct {
		text  string
		width int
		right bool
		want  string
	}{
		{text: "administrator", width: 10, want: "administr+"},
		{text: "nemo", width: 10, want: "nemo      "},
		{text: "12345678", width: 7, right: true, want: "123456+"},
		{text: "421", width: 7, right: true, want: "    421"},
		// A wide character that would straddle the cut is dropped rather than half drawn,
		// and the marker still lands in the last cell so the column holds.
		{text: "文文文文文", width: 7, want: "文文文+"},
		{text: "文", width: 1, want: ""},
	}
	for _, test := range tests {
		got := padTopCell(test.text, test.width, test.right)
		if got != test.want {
			t.Fatalf("padTopCell(%q, %d, %v) = %q, want %q",
				test.text, test.width, test.right, got, test.want)
		}
		if test.want != "" && textgrid.Cells(got) != test.width {
			t.Fatalf("padTopCell(%q, %d) is %d cells, want %d",
				test.text, test.width, textgrid.Cells(got), test.width)
		}
	}
}

// The scaled forms of TIME+, including the one that made this necessary.
func TestTopCPUTime_scalesRatherThanOverflowing(t *testing.T) {
	tests := []struct {
		used time.Duration
		want string
	}{
		{used: 0, want: "0:00.00"},
		{used: 90*time.Second + 250*time.Millisecond, want: "1:30.25"},
		{used: 424*time.Minute + 23*time.Second, want: "424:23.00"},
		// The htop form holds to ten thousand minutes, which is a week of CPU.
		{used: 9999*time.Minute + 59*time.Second, want: "9999:59.00"},
		// Past that, hours. This is Idle on a machine up thirteen days: it read
		// `264482:43.25` before, twelve cells in a column of nine.
		{used: 264482*time.Minute + 43*time.Second, want: "4408h02m"},
		// And past ten thousand hours, days.
		{used: 20000 * time.Hour, want: "833d08h"},
	}
	for _, test := range tests {
		if got := topCPUTime(test.used); got != test.want {
			t.Fatalf("topCPUTime(%v) = %q, want %q", test.used, got, test.want)
		}
	}
	column, _ := columnByKey("time")
	for _, test := range tests {
		if got := textgrid.Cells(topCPUTime(test.used)); got > column.Width {
			t.Fatalf("topCPUTime(%v) needs %d cells in a column of %d", test.used, got, column.Width)
		}
	}
}

// The default layout fits an eighty-column terminal, which is what makes it the default.
func TestTopDefaultColumns_fitEightyCells(t *testing.T) {
	columns, unknown := resolveColumns(topDefaultColumns)
	if len(unknown) != 0 {
		t.Fatalf("the default layout names columns this build has not got: %v", unknown)
	}
	total := 0
	names := []string{}
	for _, column := range columns {
		total += column.Width + 1
		names = append(names, column.Key)
	}
	// Room left for the command, which is the column anyone is actually reading.
	if room := lsDefaultWidth - total; room < 12 {
		t.Fatalf("the default layout %s leaves %d cells for COMMAND at %d columns",
			strings.Join(names, " "), room, lsDefaultWidth)
	}
}
