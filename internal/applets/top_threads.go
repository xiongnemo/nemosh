package applets

import (
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// Threads as rows, which is what `-H` and the `H` key ask for.
//
// This was the last thing in `top` that took an option and did nothing: `-H` was accepted, listed in
// `--help` and in the support matrix, sampled the thread records -- so it cost the work -- and
// produced output identical to leaving it off. Silence is the one failure mode this project does not
// allow itself; a capability that is absent has to say so. So it either had to be wired or refused,
// and the data was already being read.
//
// A thread row keeps its process in Process, so every column describing the process still works, and
// answers from the thread for the four figures a thread has of its own: its id, its priority, its
// state and its CPU. Everything else is per process, and blank on a thread row rather than repeated
// -- thirty threads each showing the process's 600 MB reads as eighteen gigabytes.

// id is the thread id on a thread row and the process id otherwise.
//
// One number for both, which is safe on this platform for a reason worth stating: Windows allocates
// process and thread ids from a single table, so a thread id can never equal a live process id. The
// cursor is remembered by this number, and on Linux that would be ambiguous.
func (r topRow) id() int {
	if r.Thread != nil {
		return r.Thread.ID
	}
	return r.Process.PID
}

func (r topRow) priority() int {
	if r.Thread != nil {
		return r.Thread.Priority
	}
	return r.Process.Priority
}

func (r topRow) state() proc.State {
	if r.Thread != nil {
		return r.Thread.State
	}
	return r.Process.State
}

func (r topRow) cpu() float64 {
	if r.Thread != nil {
		return r.ThreadRate.CPU
	}
	return r.Rate.CPU
}

func (r topRow) cpuTime() time.Duration {
	if r.Thread != nil {
		return r.Thread.Kernel + r.Thread.User
	}
	return r.Process.Kernel + r.Process.User
}

// topCellText is one cell's text, blank where a thread has no such figure.
//
// Applied here rather than in each column's formatter, so a column added later cannot forget it: the
// default for a new column is that it describes a process, and saying so is one field.
func topCellText(column topColumn, row topRow) string {
	if row.Thread != nil && column.ProcessOnly {
		return ""
	}
	return column.Cell(row)
}

// withThreads inserts each process's threads beneath it.
//
// Beneath rather than sorted in among the processes, because a thread is only meaningful next to the
// process it belongs to -- a flat list sorted by CPU would scatter one program's threads through
// four hundred rows. Threads follow the same sort as the table within their own process.
func (m topModel) withThreads(rows []topRow, rates proc.Rates) []topRow {
	if !m.Threads {
		return rows
	}
	out := make([]topRow, 0, len(rows)*2)
	for _, row := range rows {
		out = append(out, row)
		threads := make([]topRow, 0, len(row.Process.ThreadDetail))
		for index := range row.Process.ThreadDetail {
			thread := row.Process.ThreadDetail[index]
			child := row
			child.Thread = &thread
			child.ThreadRate = rates.Threads[thread.ID]
			child.Depth = row.Depth + 1
			threads = append(threads, child)
		}
		m.sortRows(threads)
		out = append(out, threads...)
	}
	return out
}
