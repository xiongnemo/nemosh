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
	// Search is the last thing searched for, which is what makes a repeat key possible: F3 and
	// n walk to the next match without asking again. Searching leaves the list whole, unlike
	// the filter -- htop is right to separate them, and this had them conflated.
	Search string
	// Tree arranges by parentage instead of sorting flat.
	Tree bool
	// Collapsed holds the pids whose children are folded away in tree mode.
	Collapsed map[int]bool
	// Tagged holds the pids `space` has marked, which is what a multiple kill acts on. htop
	// calls this tagging and it is what `space` does there -- not folding, which was the
	// binding this had wrong.
	Tagged map[int]bool
	// Paused stops the list reordering under the cursor. htop's Z, and worth having for its
	// reason: a list that moves every second cannot be read carefully, and reading carefully
	// is what someone does just before killing something.
	Paused bool
	// FullPath shows the executable's path rather than its bare name. htop's `p`, and on
	// this platform it lands exactly on the split between what needs a handle and what does
	// not.
	FullPath bool
	// Threads shows one row per thread beneath each process.
	Threads bool
	// Selected is the pid under the cursor, remembered across refreshes by identity rather
	// than by row number -- a list that reorders under a fixed cursor is how someone kills
	// the wrong process. topSelectionAbsent, not zero, means nothing is selected: pid 0 is
	// Idle and is a row you can put the cursor on.
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
		Tagged:          map[int]bool{},
		Selected:        topSelectionAbsent,
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
			Process:  process,
			Rate:     rates.Processes[process.PID],
			Details:  details.Lookup(process),
			FullPath: m.FullPath,
			Tagged:   m.Tagged[process.PID],
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
func (m topModel) matches(row topRow) bool {
	if m.Filter == "" {
		return true
	}
	return rowMatches(row, strings.ToLower(m.Filter))
}

// rowMatches is what counts as a match, for the filter and the search alike.
//
// One function for both, because htop keeps them as separate operations -- a filter hides what does
// not match, a search jumps to what does -- and two operations that disagree about what a match *is*
// would be a trap: typing the same word into each would find different processes.
//
// The command line is included because it is the only way to tell four `svchost.exe` apart, and
// telling them apart is most of why anyone searches on Windows. needle must already be lowercase.
func rowMatches(row topRow, needle string) bool {
	if strings.Contains(strings.ToLower(row.Process.Name), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(row.Details.CommandLine), needle) {
		return true
	}
	// A pid typed in should find that process, which is what someone who has a number from
	// somewhere else will try first.
	return strconv.Itoa(row.Process.PID) == needle
}

// findTopMatch is the next row matching term, starting after the given index and wrapping.
//
// Wrapping, and starting *after* rather than at, because that is what makes repeated presses walk
// through the matches instead of sticking on the first one. Pass -1 to start from the top, which is
// what an incremental search does on every keystroke.
func findTopMatch(rows []topRow, term string, after int) (int, bool) {
	if term == "" || len(rows) == 0 {
		return 0, false
	}
	needle := strings.ToLower(term)
	for offset := 1; offset <= len(rows); offset++ {
		index := ((after+offset)%len(rows) + len(rows)) % len(rows)
		if rowMatches(rows[index], needle) {
			return index, true
		}
	}
	return 0, false
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
	topActionLowerPriority
	topActionRaisePriority
	topActionHelp
	// Searching and filtering are different things, which htop is right about and this had
	// conflated: a search jumps to a match and leaves the list whole, a filter hides what does
	// not match.
	topActionSearchPrompt
	// topActionSearchNext walks to the match after the cursor, which needs the drawn row list
	// and so cannot be done by the model alone.
	topActionSearchNext
	topActionFilterPrompt
	topActionRefresh
)

// applyKey folds a key press into the model, answering what the caller must do about it.
//
// The bindings are htop's, read out of its Action.c rather than guessed at, and reading them
// corrected three things I had wrong. **`space` tags a process in htop; it does not fold a
// branch** -- folding is `+`, `-` and `=`, and getting that backwards is the kind of thing that
// makes a familiar tool feel broken. htop also has direct sort keys, `P` `M` `T` `N` for CPU,
// memory, time and pid, which are the ones a habitual user reaches for before any function key.
// And it separates *searching* from *filtering*: `/` jumps to a match and leaves the list whole,
// while `\` and F4 hide everything that does not match. This had one thing doing both.
//
// The digits are top's convention rather than htop's, and are kept alongside because this is
// named `top` and someone typing `1` should get something.
//
// htop is GPL-2.0, so it is read for behaviour and nothing is copied from it -- the same standing
// busybox has here. See docs/design/reference-methodology.md.
func (m *topModel) applyKey(key string) topAction {
	switch key {
	case "q", "esc":
		return topActionQuit
	case "F9", "k":
		return topActionKill
	case "F7", "[":
		return topActionLowerPriority
	case "F8", "]":
		return topActionRaisePriority
	case "F1", "h", "?":
		return topActionHelp
	case "/":
		return topActionSearchPrompt
	case "F3":
		// htop's F3 is "search next" once there is something to search for, and asking
		// again for a term you have already typed is the wrong answer to that key.
		if m.Search == "" {
			return topActionSearchPrompt
		}
		return topActionSearchNext
	case "n":
		return topActionSearchNext
	case "F4", "\\":
		return topActionFilterPrompt
	case "F5", "t":
		m.Tree = !m.Tree
		return topActionNone
	case "space":
		// Tag, as htop does. Tagged processes are what a multiple kill acts on.
		if m.Selected != topSelectionAbsent {
			if m.Tagged == nil {
				m.Tagged = map[int]bool{}
			}
			m.Tagged[m.Selected] = !m.Tagged[m.Selected]
		}
		return topActionNone
	case "U":
		m.Tagged = map[int]bool{}
		return topActionNone
	case "+", "-", "=":
		if m.Tree && m.Selected != topSelectionAbsent {
			m.Collapsed[m.Selected] = !m.Collapsed[m.Selected]
		}
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
	case "Z":
		// Pausing is worth having for the reason htop has it: a list that reorders every
		// second cannot be read carefully, and reading carefully is what someone does
		// just before killing something.
		m.Paused = !m.Paused
		return topActionNone
	case "p":
		// Full path against bare name -- htop's `p`, and it lands on exactly the
		// distinction this platform forces: the path needs a handle and the name never
		// does. See internal/proc/detail_windows.go.
		m.FullPath = !m.FullPath
		return topActionNone
	case "P":
		m.sortBy("cpu")
		return topActionNone
	case "M":
		m.sortBy("mem")
		return topActionNone
	case "T":
		m.sortBy("time")
		return topActionNone
	case "N":
		m.sortBy("pid")
		return topActionNone
	case "r":
		return topActionRefresh
	}
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
	m.setSort(key)
}

// setSort selects a column and the direction it wants, without the toggle sortBy applies.
//
// Separate from sortBy because the command line needs it: `top -s pid` must get pid's own
// direction, and routing that through sortBy would have reversed it whenever the requested column
// happened to be the one already selected.
func (m *topModel) setSort(key string) {
	m.Sort = key
	m.Descending = true
	if column, ok := columnByKey(key); ok {
		m.Descending = !column.Ascending
	}
}
