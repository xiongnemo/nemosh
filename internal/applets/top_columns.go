package applets

import (
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// The columns, as data rather than as code.
//
// A table declared this way can be reordered by a configuration file, sorted by any of its
// entries and rendered by something that knows nothing about processes, none of which is true of
// a hand-written row formatter. It is also the only way the sort stays honest: the comparator
// lives beside the formatter, so a column cannot end up sorting by one thing and displaying
// another.

// topRow is one process as the view sees it: the sample, plus what was derived, plus what could
// only be learned by asking.
type topRow struct {
	Process proc.Process
	Rate    proc.ProcessRate
	Details proc.Details
	// FullPath asks for the executable's path rather than its bare name -- htop's `p`, and
	// on this platform the toggle between what a handle answered and what the table always
	// knows.
	FullPath bool
	// Tagged marks a row `space` has picked out.
	Tagged bool
	// Depth is the indent in tree mode, and zero in every other.
	Depth int
	// MemoryShare is the working set over the machine's physical memory.
	MemoryShare float64
}

// topColumn is one column: what it is called, how wide, how to render a row, and how to order
// two rows by it.
type topColumn struct {
	Key    string
	Header string
	Width  int
	// Right says the cell is right-aligned, which every number is and no name is.
	Right bool
	// Ascending says a first press sorts this column upwards.
	//
	// The rule is the difference between a magnitude and a name. Sorting by CPU or memory is
	// asking "what is using the most", so the largest comes first; sorting by pid or by command
	// is asking to read the list in order, so it goes in reading order. Without this, pressing
	// N put pid 65000 at the top and looked as though nothing useful had happened.
	Ascending bool
	Cell      func(row topRow) string
	// Less orders two rows ascending. Sorting descending is the view's business, not the
	// column's, so there is one comparator per column rather than two.
	Less func(a, b topRow) bool
}

// topColumns is every column this can show, in the order a default layout uses.
//
// Windows-only extras are in here on purpose. HANDLES has no POSIX counterpart and is one of the
// most useful numbers on this platform -- a handle leak is the commonest way a Windows process
// misbehaves, and nothing else in the shell can show it.
var topColumns = []topColumn{
	{
		Key: "pid", Header: "PID", Width: 7, Right: true, Ascending: true,
		Cell: func(r topRow) string { return strconv.Itoa(r.Process.PID) },
		Less: func(a, b topRow) bool { return a.Process.PID < b.Process.PID },
	},
	{
		Key: "ppid", Header: "PPID", Width: 7, Right: true, Ascending: true,
		Cell: func(r topRow) string { return strconv.Itoa(r.Process.PPID) },
		Less: func(a, b topRow) bool { return a.Process.PPID < b.Process.PPID },
	},
	{
		Key: "user", Header: "USER", Width: 10, Ascending: true,
		// Empty rather than a guess where the handle was refused; the header stays so the
		// reader can see the column exists and that this row would not answer.
		Cell: func(r topRow) string { return r.Details.User },
		Less: func(a, b topRow) bool { return a.Details.User < b.Details.User },
	},
	{
		Key: "pri", Header: "PRI", Width: 5,
		Cell: func(r topRow) string { return topPriority(r.Process.Priority) },
		Less: func(a, b topRow) bool { return a.Process.Priority < b.Process.Priority },
	},
	{
		Key: "state", Header: "S", Width: 1, Ascending: true,
		Cell: func(r topRow) string { return string(rune(r.Process.State)) },
		Less: func(a, b topRow) bool { return a.Process.State < b.Process.State },
	},
	{
		Key: "cpu", Header: "CPU%", Width: 5, Right: true,
		Cell: func(r topRow) string { return topPercent(r.Rate.CPU) },
		Less: func(a, b topRow) bool { return a.Rate.CPU < b.Rate.CPU },
	},
	{
		Key: "mem", Header: "MEM%", Width: 5, Right: true,
		Cell: func(r topRow) string { return topPercent(r.MemoryShare) },
		Less: func(a, b topRow) bool { return a.MemoryShare < b.MemoryShare },
	},
	{
		Key: "rss", Header: "RSS", Width: 6, Right: true,
		Cell: func(r topRow) string { return topBytes(r.Process.WorkingSet) },
		Less: func(a, b topRow) bool { return a.Process.WorkingSet < b.Process.WorkingSet },
	},
	{
		Key: "private", Header: "PRIV", Width: 6, Right: true,
		Cell: func(r topRow) string { return topBytes(r.Process.PrivateWorkingSet) },
		Less: func(a, b topRow) bool { return a.Process.PrivateWorkingSet < b.Process.PrivateWorkingSet },
	},
	{
		Key: "commit", Header: "COMMIT", Width: 6, Right: true,
		Cell: func(r topRow) string { return topBytes(r.Process.Commit) },
		Less: func(a, b topRow) bool { return a.Process.Commit < b.Process.Commit },
	},
	{
		Key: "thr", Header: "THR", Width: 4, Right: true,
		Cell: func(r topRow) string { return strconv.Itoa(r.Process.Threads) },
		Less: func(a, b topRow) bool { return a.Process.Threads < b.Process.Threads },
	},
	{
		Key: "handles", Header: "HND", Width: 6, Right: true,
		Cell: func(r topRow) string { return strconv.Itoa(r.Process.Handles) },
		Less: func(a, b topRow) bool { return a.Process.Handles < b.Process.Handles },
	},
	{
		Key: "read", Header: "READ/s", Width: 7, Right: true,
		Cell: func(r topRow) string { return topRate(r.Rate.ReadBytesPerSecond) },
		Less: func(a, b topRow) bool { return a.Rate.ReadBytesPerSecond < b.Rate.ReadBytesPerSecond },
	},
	{
		Key: "write", Header: "WRITE/s", Width: 7, Right: true,
		Cell: func(r topRow) string { return topRate(r.Rate.WriteBytesPerSecond) },
		Less: func(a, b topRow) bool { return a.Rate.WriteBytesPerSecond < b.Rate.WriteBytesPerSecond },
	},
	{
		Key: "time", Header: "TIME+", Width: 9, Right: true,
		Cell: func(r topRow) string { return topCPUTime(r.Process.Kernel + r.Process.User) },
		Less: func(a, b topRow) bool {
			return a.Process.Kernel+a.Process.User < b.Process.Kernel+b.Process.User
		},
	},
	{
		Key: "command", Header: "COMMAND", Width: 0, Ascending: true,
		// Width zero means the rest of the line. The command is last for that reason, and
		// because it is the only column whose length nobody can predict.
		Cell: func(r topRow) string {
			indent := strings.Repeat(" ", r.Depth*2)
			tag := ""
			if r.Tagged {
				tag = "* "
			}
			if r.FullPath {
				return indent + tag + r.Details.Command(r.Process.Name)
			}
			// The bare name by default, because a full Chrome command line is two
			// thousand characters and buries every other row's name.
			return indent + tag + r.Process.Name
		},
		Less: func(a, b topRow) bool { return a.Process.Name < b.Process.Name },
	},
}

// topDefaultColumns is the layout used when no configuration says otherwise.
//
// Not every column: thirteen of them do not fit on an eighty-column terminal, and a default that
// needs a wide window is a default that looks broken. The rest are available by configuration.
var topDefaultColumns = []string{"pid", "user", "pri", "state", "cpu", "mem", "rss", "thr", "time", "command"}

// columnByKey finds a column by name, which is how a configuration file names one.
func columnByKey(key string) (topColumn, bool) {
	for _, column := range topColumns {
		if column.Key == key {
			return column, true
		}
	}
	return topColumn{}, false
}

// resolveColumns turns a list of keys into columns, ignoring names this build does not have.
//
// Ignoring rather than refusing, because a configuration file written against a later version
// should degrade to a working table rather than to an error at startup. Anything unknown is
// reported to the caller so it can be said once rather than guessed at.
func resolveColumns(keys []string) ([]topColumn, []string) {
	var columns []topColumn
	var unknown []string
	for _, key := range keys {
		if column, ok := columnByKey(key); ok {
			columns = append(columns, column)
			continue
		}
		unknown = append(unknown, key)
	}
	if len(columns) == 0 {
		columns, _ = resolveColumns(topDefaultColumns)
	}
	return columns, unknown
}
