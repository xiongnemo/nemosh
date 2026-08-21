package applets

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/xiongnemo/nemosh/internal/proc"
	"github.com/xiongnemo/nemosh/internal/textgrid"
)

// The plain form: what `top` prints when nothing is watching interactively.
//
// This is the whole of what a script can use and the whole of what a test can check, so it is
// written first and the interactive view is drawn from the same rows. A monitor whose two forms
// disagree is a monitor with two bugs.
//
// `-n 1` is the default and reports zero for every CPU percentage, because one sample has nothing
// to compare against. That is not a defect to paper over -- top reports the same, and a first
// sample showing each process's whole lifetime of CPU as though it were spent in the last second
// would be worse than a column of zeroes. `-n 2` is the shortest useful run.

// runTopBatch prints samples and exits.
func runTopBatch(ctx context.Context, options topOptions, stdout io.Writer) error {
	session := newTopSession(options)
	for iteration := 0; iteration < options.iterations; iteration++ {
		if iteration > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(options.delay):
			}
			if _, err := fmt.Fprintln(stdout); err != nil {
				return err
			}
		}
		snapshot, rates, rows, err := session.sample()
		if err != nil {
			return err
		}
		if err := writeTopSummary(stdout, snapshot, rates); err != nil {
			return err
		}
		if err := writeTopTable(stdout, session.model.Columns, rows, 0, topBatchWidth(stdout)); err != nil {
			return err
		}
	}
	return nil
}

// writeTopSummary is the header: what the machine is doing as a whole.
//
// No load average, and that is a decision rather than an omission. Windows has no such measure --
// there is no runnable-thread average anywhere in the kernel -- and the nearest analogue, the
// processor queue length, counts something else. Printing a number under that name would be
// worse than printing nothing, so `--help` says why instead. It is the same posture `ps` takes
// for TTY.
func writeTopSummary(stdout io.Writer, snapshot proc.Snapshot, rates proc.Rates) error {
	running := 0
	threads := 0
	for _, process := range snapshot.Processes {
		threads += process.Threads
		if process.State == proc.StateRunning {
			running++
		}
	}
	if _, err := fmt.Fprintf(stdout, "top - up %s, %d processes, %d threads, %d running\n",
		topUptime(snapshot.Uptime), len(snapshot.Processes), threads, running); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "CPU: %s%% busy over %d processors\n",
		strings.TrimSpace(topPercent(rates.TotalBusy)), len(snapshot.CPUs)); err != nil {
		return err
	}
	memory := snapshot.Memory
	if _, err := fmt.Fprintf(stdout, "Mem: %s used of %s, %s cached\n",
		topBytes(memory.UsedPhysical()), topBytes(memory.TotalPhysical), topBytes(memory.Cached)); err != nil {
		return err
	}
	// Commit rather than swap: Windows promises memory rather than paging it out in a way
	// that can be counted the way Linux counts swap. Naming it commit keeps the difference
	// visible instead of putting a different measure under a familiar label.
	_, err := fmt.Fprintf(stdout, "Commit: %s of %s\n",
		topBytes(memory.CommitTotal), topBytes(memory.CommitLimit))
	return err
}

// writeTopTable prints the header row and the rows beneath it.
//
// limit caps how many rows are printed, and zero means all of them -- the interactive view knows
// how tall the terminal is and the batch form does not care.
func writeTopTable(stdout io.Writer, columns []topColumn, rows []topRow, limit, width int) error {
	if _, err := fmt.Fprintln(stdout, truncateToCells(topHeaderLine(columns), width)); err != nil {
		return err
	}
	for index, row := range rows {
		if limit > 0 && index >= limit {
			break
		}
		if _, err := fmt.Fprintln(stdout, truncateToCells(topRowLine(columns, row), width)); err != nil {
			return err
		}
	}
	return nil
}

// topBatchWidth is how wide a plain line may be.
//
// Truncated, because a command line is not a bounded thing: a Chrome renderer's is two thousand
// characters and a shell's `-c` argument can be longer, so an untruncated table is one row per
// screenful. top truncates to the terminal too, and offers `-w` to widen; this takes the
// terminal's width where there is one and 80 where there is not, which is what a pipe gets from
// every other tool here.
func topBatchWidth(stdout io.Writer) int {
	if file := stdoutFile(stdout); file != nil {
		if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 0 {
			return width
		}
	}
	return lsDefaultWidth
}

// topHeaderLine is the column headers, aligned as their cells will be.
func topHeaderLine(columns []topColumn) string {
	cells := make([]string, 0, len(columns))
	for _, column := range columns {
		cells = append(cells, padTopCell(column.Header, column.Width, column.Right))
	}
	return strings.Join(cells, " ")
}

// topRowLine is one process as a line of text.
func topRowLine(columns []topColumn, row topRow) string {
	cells := make([]string, 0, len(columns))
	for _, column := range columns {
		cells = append(cells, padTopCell(column.Cell(row), column.Width, column.Right))
	}
	return strings.Join(cells, " ")
}

// padTopCell fits text to a column, measured in terminal cells.
//
// textgrid rather than len, because a process can be called anything a filesystem allows and a
// CJK name is twice as wide as it is long -- the same reason `ls` needed it. A width of zero
// means the column takes what it needs, which only the last one may do.
func padTopCell(text string, width int, right bool) string {
	if width <= 0 {
		return text
	}
	cells := textgrid.Cells(text)
	if cells > width {
		return truncateToCells(text, width)
	}
	padding := strings.Repeat(" ", width-cells)
	if right {
		return padding + text
	}
	return text + padding
}

// truncateToCells cuts text to fit, counting cells rather than runes. A width of zero or less
// means no limit.
//
// A wide character that would straddle the boundary is dropped rather than half-drawn: a terminal
// asked to draw half of one draws something arbitrary, and the column after it moves.
func truncateToCells(text string, width int) string {
	if width <= 0 {
		return text
	}
	used := 0
	for index, r := range text {
		size := textgrid.RuneCells(r)
		if used+size > width {
			return text[:index]
		}
		used += size
	}
	return text
}
