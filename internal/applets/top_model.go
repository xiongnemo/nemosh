package applets

import (
	"sort"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// The view, with no terminal in it.
//
// Everything a key press does to a monitor -- sort by another column, reverse it, filter, fold a
// branch, follow a process, move the selection -- is a change to a few fields and a recomputation
// of a list. Keeping that here means it can be tested by pressing keys at a struct, which is the
// only way any of it gets tested at all: a tview application needs a terminal, and a terminal is
// the one thing a test has not got.

// topModel is what the view is showing and how.
type topModel struct {
	// Columns is the layout, resolved once from configuration.
	Columns []topColumn
	// Sort is the key of the column being sorted on, and Descending which way. Descending is
	// the default because the interesting processes are the busy ones.
	Sort       string
	Descending bool
	// Filter narrows the list to names and command lines containing it, case-insensitively.
	Filter string
	// Tree arranges by parentage instead of sorting flat.
	Tree bool
	// Collapsed holds the pids whose children are folded away in tree mode.
	Collapsed map[int]bool
	// Threads shows one row per thread beneath each process.
	Threads bool
	// Selected is the pid under the cursor, remembered across refreshes by identity rather
	// than by row number -- a list that reorders under a fixed cursor is how someone kills
	// the wrong process.
	Selected int
	// KernelProcesses includes PID 0 and 4, which htop hides behind a toggle and which are
	// worth seeing on Windows: Idle holds the machine's spare capacity and System holds its
	// driver threads.
	KernelProcesses bool
}

// newTopModel is the default view: sorted by CPU, busiest first.
func newTopModel(columns []topColumn) topModel {
	return topModel{
		Columns:         columns,
		Sort:            "cpu",
		Descending:      true,
		Collapsed:       map[int]bool{},
		KernelProcesses: true,
	}
}

// rows turns a snapshot and its rates into the list to draw.
//
// One function for every arrangement the view can be in, because the alternative is four
// functions that each get the filter subtly differently.
func (m topModel) rows(snapshot proc.Snapshot, rates proc.Rates, details *proc.DetailCache) []topRow {
	rows := make([]topRow, 0, len(snapshot.Processes))
	for _, process := range snapshot.Processes {
		if !m.KernelProcesses && (process.PID == 0 || process.PID == 4) {
			continue
		}
		row := topRow{
			Process: process,
			Rate:    rates.Processes[process.PID],
			Details: details.Lookup(process),
		}
		if snapshot.Memory.TotalPhysical > 0 {
			row.MemoryShare = float64(process.WorkingSet) / float64(snapshot.Memory.TotalPhysical)
		}
		if !m.matches(row) {
			continue
		}
		rows = append(rows, row)
	}
	if m.Tree {
		return m.treeRows(rows)
	}
	m.sortRows(rows)
	return rows
}

// matches is the filter: a substring of the name or of the command line, case-insensitively.
//
// The command line is included because it is the only way to tell four `svchost.exe` apart, and
// telling them apart is most of why anyone filters on Windows.
func (m topModel) matches(row topRow) bool {
	if m.Filter == "" {
		return true
	}
	needle := strings.ToLower(m.Filter)
	if strings.Contains(strings.ToLower(row.Process.Name), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(row.Details.CommandLine), needle) {
		return true
	}
	// A pid typed into the filter should find that process, which is what someone who has a
	// number from somewhere else will try first.
	return strconv.Itoa(row.Process.PID) == m.Filter
}

// sortRows orders the list by the chosen column.
func (m topModel) sortRows(rows []topRow) {
	column, ok := columnByKey(m.Sort)
	if !ok {
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if m.Descending {
			return column.Less(rows[j], rows[i])
		}
		return column.Less(rows[i], rows[j])
	})
}

// treeRows arranges by parentage, ordering siblings by the chosen column.
//
// The filter is applied *before* the tree is built, which is a decision worth stating: a filtered
// tree shows the matches as roots rather than showing their ancestors as context. htop keeps the
// ancestors; this does not, because on Windows a parent is often gone and the chain to it is
// noise rather than context.
func (m topModel) treeRows(rows []topRow) []topRow {
	byPID := make(map[int]topRow, len(rows))
	processes := make([]proc.Process, 0, len(rows))
	for _, row := range rows {
		byPID[row.Process.PID] = row
		processes = append(processes, row.Process)
	}
	column, ok := columnByKey(m.Sort)
	less := func(a, b proc.Process) bool { return a.PID < b.PID }
	if ok {
		less = func(a, b proc.Process) bool {
			left, right := byPID[a.PID], byPID[b.PID]
			if m.Descending {
				return column.Less(right, left)
			}
			return column.Less(left, right)
		}
	}
	nodes := proc.Flatten(proc.Tree(processes, less), m.Collapsed)
	arranged := make([]topRow, 0, len(nodes))
	for _, node := range nodes {
		row := byPID[node.Process.PID]
		row.Depth = node.Depth
		arranged = append(arranged, row)
	}
	return arranged
}

// topAction is what a key press asked for, where it is not simply a change of state.
type topAction int

const (
	topActionNone topAction = iota
	topActionQuit
	topActionKill
	topActionNice
	topActionHelp
	topActionFilterPrompt
	topActionRefresh
)

// applyKey folds a key press into the model, answering what the caller must do about it.
//
// A key that only changes how the list is arranged is handled entirely here and answers None,
// which is what keeps the rendering layer from growing a copy of the view's rules.
func (m *topModel) applyKey(key string) topAction {
	switch key {
	case "q", "esc":
		return topActionQuit
	case "F9", "k":
		return topActionKill
	case "F7", "F8":
		return topActionNice
	case "F1", "h", "?":
		return topActionHelp
	case "F3", "/":
		return topActionFilterPrompt
	case "F5", "t":
		m.Tree = !m.Tree
		return topActionNone
	case "H":
		m.Threads = !m.Threads
		return topActionNone
	case "K":
		m.KernelProcesses = !m.KernelProcesses
		return topActionNone
	case "I":
		m.Descending = !m.Descending
		return topActionNone
	case "space":
		if m.Tree && m.Selected != 0 {
			m.Collapsed[m.Selected] = !m.Collapsed[m.Selected]
		}
		return topActionNone
	case "r":
		return topActionRefresh
	}
	// A digit selects a column to sort by, which is how top does it and is faster than
	// hunting for a function key.
	if index, err := strconv.Atoi(key); err == nil && index >= 1 && index <= len(m.Columns) {
		m.sortBy(m.Columns[index-1].Key)
		return topActionNone
	}
	return topActionNone
}

// sortBy switches the sort column, or reverses the order when it is already that column.
//
// Reversing on a second press is what every table in every program does, and doing anything else
// here would be a surprise for no gain.
func (m *topModel) sortBy(key string) {
	if m.Sort == key {
		m.Descending = !m.Descending
		return
	}
	m.Sort = key
	// A new column starts descending, because the reason to sort by a column is almost always
	// to see the largest values in it.
	m.Descending = true
}
