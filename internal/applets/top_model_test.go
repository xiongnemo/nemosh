package applets

import (
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// Everything a key press does to the view, tested by pressing keys at a struct.
//
// This is the whole reason the model has no terminal in it. A tview application needs a console,
// and a console is the one thing a test has not got -- so if the sorting, filtering and folding
// lived in the widget callbacks, none of it would be checked by anything but a person looking at a
// screen.

var modelEpoch = time.Date(2026, time.August, 21, 12, 0, 0, 0, time.UTC)

func testProcess(pid, ppid int, name string, working uint64) proc.Process {
	return proc.Process{
		PID: pid, PPID: ppid, Name: name, WorkingSet: working,
		Created: modelEpoch.Add(time.Duration(pid) * time.Second), Threads: 1,
	}
}

// testRows builds a snapshot and its rates so the model can be asked for rows.
func testSnapshot(processes ...proc.Process) (proc.Snapshot, proc.Rates) {
	snapshot := proc.Snapshot{
		Taken:     modelEpoch,
		Processes: processes,
		CPUs:      make([]proc.CPUTime, 1),
		Memory:    proc.Memory{TotalPhysical: 1 << 30},
	}
	rates := proc.Rates{Interval: time.Second, Processes: map[int]proc.ProcessRate{}}
	for index, process := range processes {
		// A descending CPU order that is *not* the pid order, so a sort that quietly does
		// nothing cannot pass.
		rates.Processes[process.PID] = proc.ProcessRate{CPU: float64(len(processes)-index) / 100}
	}
	return snapshot, rates
}

func rowNames(rows []topRow) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Process.Name)
	}
	return names
}

func assertNames(t *testing.T, rows []topRow, want ...string) {
	t.Helper()
	got := rowNames(rows)
	if len(got) != len(want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("rows = %v, want %v", got, want)
		}
	}
}

func TestTopModel_sortsByCPUDescendingByDefault(t *testing.T) {
	model := newTopModel(mustColumns(t))
	snapshot, rates := testSnapshot(
		testProcess(1, 0, "first", 100),
		testProcess(2, 0, "second", 200),
		testProcess(3, 0, "third", 300),
	)

	// When
	rows := model.rows(snapshot, rates, proc.NewDetailCache())

	// Then -- busiest first, which is the only default that makes a monitor useful at a glance
	assertNames(t, rows, "first", "second", "third")
}

func TestTopModel_sortByReversesOnASecondPress(t *testing.T) {
	model := newTopModel(mustColumns(t))

	// When
	model.sortBy("rss")

	// Then -- a new column starts descending, because the reason to sort by one is to see its
	// largest values
	if model.Sort != "rss" || !model.Descending {
		t.Fatalf("sort = %q descending = %v, want rss descending", model.Sort, model.Descending)
	}

	// When pressed again
	model.sortBy("rss")

	// Then
	if model.Descending {
		t.Fatal("a second press on the same column did not reverse the order")
	}
}

func TestTopModel_sortsByTheChosenColumn(t *testing.T) {
	model := newTopModel(mustColumns(t))
	model.sortBy("rss")
	snapshot, rates := testSnapshot(
		testProcess(1, 0, "small", 100),
		testProcess(2, 0, "large", 900),
		testProcess(3, 0, "middle", 500),
	)

	// When
	rows := model.rows(snapshot, rates, proc.NewDetailCache())

	// Then
	assertNames(t, rows, "large", "middle", "small")
}

func TestTopModel_filterMatchesNameAndPID(t *testing.T) {
	snapshot, rates := testSnapshot(
		testProcess(11, 0, "chrome.exe", 100),
		testProcess(12, 0, "nemosh.exe", 100),
		testProcess(13, 0, "CHROME.EXE", 100),
	)
	tests := []struct {
		filter string
		want   []string
	}{
		// Case-insensitively, because nobody types a Windows executable's case correctly.
		{filter: "chrome", want: []string{"chrome.exe", "CHROME.EXE"}},
		{filter: "nemosh", want: []string{"nemosh.exe"}},
		// A pid typed in, which is what someone with a number from elsewhere tries first.
		{filter: "12", want: []string{"nemosh.exe"}},
		{filter: "nothing here", want: nil},
	}
	for _, test := range tests {
		t.Run(test.filter, func(t *testing.T) {
			model := newTopModel(mustColumns(t))
			model.Filter = test.filter

			// When
			rows := model.rows(snapshot, rates, proc.NewDetailCache())

			// Then
			assertNames(t, rows, test.want...)
		})
	}
}

func TestTopModel_treeShowsParentage(t *testing.T) {
	model := newTopModel(mustColumns(t))
	model.Tree = true
	snapshot, rates := testSnapshot(
		testProcess(1, 0, "root", 100),
		testProcess(2, 1, "child", 100),
		testProcess(3, 2, "grandchild", 100),
	)

	// When
	rows := model.rows(snapshot, rates, proc.NewDetailCache())

	// Then
	assertNames(t, rows, "root", "child", "grandchild")
	if rows[0].Depth != 0 || rows[1].Depth != 1 || rows[2].Depth != 2 {
		t.Fatalf("depths = %d %d %d, want 0 1 2", rows[0].Depth, rows[1].Depth, rows[2].Depth)
	}
}

func TestTopModel_foldingABranchHidesItsChildren(t *testing.T) {
	model := newTopModel(mustColumns(t))
	model.Tree = true
	model.Selected = 2
	snapshot, rates := testSnapshot(
		testProcess(1, 0, "root", 100),
		testProcess(2, 1, "folded", 100),
		testProcess(3, 2, "hidden", 100),
	)

	// When -- `-` folds the selected branch. Not space: space *tags* in htop, and having
	// space fold was the binding this got wrong before its Action.c was read.
	if action := model.applyKey("-"); action != topActionNone {
		t.Fatalf("- asked for %v, want nothing but a redraw", action)
	}

	// Then
	assertNames(t, model.rows(snapshot, rates, proc.NewDetailCache()), "root", "folded")
}

func TestTopModel_kernelProcessesCanBeHidden(t *testing.T) {
	model := newTopModel(mustColumns(t))
	snapshot, rates := testSnapshot(
		testProcess(0, 0, "Idle", 0),
		testProcess(4, 0, "System", 0),
		testProcess(500, 4, "svchost.exe", 100),
	)

	// When
	model.applyKey("K")

	// Then -- Idle and System are the two worth a toggle: Idle holds the machine's spare
	// capacity, and on an idle machine it is the top row for ever.
	assertNames(t, model.rows(snapshot, rates, proc.NewDetailCache()), "svchost.exe")
}

func TestTopModel_keysThatAskTheCallerForSomething(t *testing.T) {
	tests := []struct {
		key  string
		want topAction
	}{
		{key: "q", want: topActionQuit},
		{key: "esc", want: topActionQuit},
		{key: "F9", want: topActionKill},
		{key: "k", want: topActionKill},
		// htop separates these and this had them conflated: `/` searches, leaving the
		// list whole, and F4 or backslash filters, hiding what does not match.
		{key: "F3", want: topActionSearchPrompt},
		{key: "/", want: topActionSearchPrompt},
		{key: "F4", want: topActionFilterPrompt},
		{key: "F7", want: topActionLowerPriority},
		{key: "[", want: topActionLowerPriority},
		{key: "F8", want: topActionRaisePriority},
		{key: "]", want: topActionRaisePriority},
		{key: "F1", want: topActionHelp},
		{key: "r", want: topActionRefresh},
		// Handled entirely by the model, so the caller is told to do nothing but redraw.
		{key: "F5", want: topActionNone},
		{key: "H", want: topActionNone},
		{key: "I", want: topActionNone},
		{key: "1", want: topActionNone},
		{key: "space", want: topActionNone},
		{key: "Z", want: topActionNone},
		{key: "p", want: topActionNone},
		{key: "P", want: topActionNone},
		// Not a key this knows, and not an error either: tview's own table keys arrive
		// here too and must be passed through untouched.
		{key: "z", want: topActionNone},
	}
	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			model := newTopModel(mustColumns(t))

			// When
			got := model.applyKey(test.key)

			// Then
			if got != test.want {
				t.Fatalf("applyKey(%q) = %v, want %v", test.key, got, test.want)
			}
		})
	}
}

func TestTopModel_digitSortsByThatColumn(t *testing.T) {
	model := newTopModel(mustColumns(t))

	// When -- the first column is pid in the default layout
	model.applyKey("1")

	// Then
	if model.Sort != "pid" {
		t.Fatalf("sort = %q, want pid after pressing 1", model.Sort)
	}

	// And a digit past the end of the layout changes nothing rather than panicking.
	model.applyKey("99")
	if model.Sort != "pid" {
		t.Fatalf("sort = %q, want it unchanged by an out-of-range digit", model.Sort)
	}
}

func TestResolveColumns_unknownNameFallsBackRatherThanFailing(t *testing.T) {
	// A configuration file written against a later version should still produce a table.
	columns, unknown := resolveColumns([]string{"pid", "not-a-column", "cpu"})

	// Then
	if len(columns) != 2 || columns[0].Key != "pid" || columns[1].Key != "cpu" {
		t.Fatalf("columns = %v, want pid and cpu", columnKeys(columns))
	}
	if len(unknown) != 1 || unknown[0] != "not-a-column" {
		t.Fatalf("unknown = %v, want the one bad name reported", unknown)
	}

	// And a layout with nothing usable in it falls back to the default rather than to an
	// empty table, which would be a monitor showing nothing at all.
	fallback, _ := resolveColumns([]string{"nope"})
	if len(fallback) != len(topDefaultColumns) {
		t.Fatalf("fallback = %v, want the default layout", columnKeys(fallback))
	}
}

func mustColumns(t *testing.T) []topColumn {
	t.Helper()
	columns, unknown := resolveColumns(topDefaultColumns)
	if len(unknown) != 0 {
		t.Fatalf("the default layout names columns this build has not got: %v", unknown)
	}
	return columns
}

func columnKeys(columns []topColumn) []string {
	keys := make([]string, 0, len(columns))
	for _, column := range columns {
		keys = append(keys, column.Key)
	}
	return keys
}

// The bindings htop is known for, which this had wrong or missing until its Action.c was read.
func TestTopModel_htopBindings(t *testing.T) {
	t.Run("space tags rather than folding", func(t *testing.T) {
		model := newTopModel(mustColumns(t))
		model.Tree = true
		model.Selected = 42

		// When
		model.applyKey("space")

		// Then -- tagged, and the branch is *not* folded
		if !model.Tagged[42] {
			t.Fatal("space did not tag the selected process")
		}
		if model.Collapsed[42] {
			t.Fatal("space folded the branch; that is what +, - and = do")
		}

		// And U clears every tag, which is how htop undoes a wrong selection.
		model.applyKey("U")
		if len(model.Tagged) != 0 {
			t.Fatalf("U left %d tags", len(model.Tagged))
		}
	})

	t.Run("direct sort keys", func(t *testing.T) {
		tests := map[string]string{"P": "cpu", "M": "mem", "T": "time", "N": "pid"}
		for key, want := range tests {
			model := newTopModel(mustColumns(t))

			// When
			model.applyKey(key)

			// Then -- these are the keys a habitual user reaches for before any
			// function key, and none of them existed here before.
			if model.Sort != want {
				t.Fatalf("%s sorted by %q, want %q", key, model.Sort, want)
			}
		}
	})

	t.Run("Z pauses and p switches to the full path", func(t *testing.T) {
		model := newTopModel(mustColumns(t))

		// When
		model.applyKey("Z")
		model.applyKey("p")

		// Then
		if !model.Paused {
			t.Fatal("Z did not pause")
		}
		if !model.FullPath {
			t.Fatal("p did not switch to the full path")
		}
	})
}

// Which way a first press sorts, which is the difference between a useful key and one that looks
// broken. Pressing N put pid 65000 at the top before this: sorting by pid had always worked, and
// had always answered a question nobody asks.
func TestTopModel_aFirstPressSortsTheWayTheColumnWants(t *testing.T) {
	tests := []struct {
		key        string
		want       string
		descending bool
	}{
		// Magnitudes: the reason to sort by one is to see the largest.
		{key: "P", want: "cpu", descending: true},
		{key: "M", want: "mem", descending: true},
		{key: "T", want: "time", descending: true},
		// Names and identities: the reason to sort by one is to read the list in order.
		{key: "N", want: "pid", descending: false},
	}
	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			model := newTopModel(mustColumns(t))
			// Sorted by something else first, so this is a first press rather than a
			// second one. A second press on the selected column reverses it, which is
			// right and is what this would otherwise be measuring: the default sort is
			// already cpu, so pressing P from a fresh model reverses instead of choosing.
			model.setSort("handles")

			// When
			model.applyKey(test.key)

			// Then
			if model.Sort != test.want || model.Descending != test.descending {
				t.Fatalf("%s sorted by %q descending=%v, want %q descending=%v",
					test.key, model.Sort, model.Descending, test.want, test.descending)
			}
		})
	}
}

func TestTopModel_sortsByPIDInReadingOrder(t *testing.T) {
	model := newTopModel(mustColumns(t))
	snapshot, rates := testSnapshot(
		testProcess(900, 0, "last", 100),
		testProcess(4, 0, "first", 100),
		testProcess(500, 0, "middle", 100),
	)

	// When
	model.applyKey("N")

	// Then
	assertNames(t, model.rows(snapshot, rates, proc.NewDetailCache()), "first", "middle", "last")

	// And a second press reverses it, as it does for any column.
	model.applyKey("N")
	assertNames(t, model.rows(snapshot, rates, proc.NewDetailCache()), "last", "middle", "first")
}

// `-s pid` gets pid's direction too, which is why setSort exists apart from sortBy.
func TestTopSession_theRequestedSortKeepsItsOwnDirection(t *testing.T) {
	tests := map[string]bool{"pid": false, "cpu": true, "rss": true, "command": false}
	for key, descending := range tests {
		options, err := topArgs([]string{"-s", key})
		if err != nil {
			t.Fatalf("topArgs -s %s: %v", key, err)
		}

		// When
		session := newTopSession(options)

		// Then
		if session.model.Sort != key || session.model.Descending != descending {
			t.Fatalf("-s %s gave sort=%q descending=%v, want descending=%v",
				key, session.model.Sort, session.model.Descending, descending)
		}
	}
}

// Searching: which row the cursor should land on. The walking is pure, so it is tested here rather
// than through a terminal.
func TestFindTopMatch_walksThroughTheMatches(t *testing.T) {
	rows := []topRow{
		{Process: proc.Process{PID: 1, Name: "Idle"}},
		{Process: proc.Process{PID: 2, Name: "svchost.exe"}},
		{Process: proc.Process{PID: 3, Name: "chrome.exe"}},
		{Process: proc.Process{PID: 4, Name: "SVCHOST.EXE"}},
	}
	tests := []struct {
		name  string
		term  string
		after int
		want  int
		found bool
	}{
		// From the top, which is what every keystroke of an incremental search does.
		{name: "from the top", term: "svchost", after: -1, want: 1, found: true},
		// After the cursor, which is what makes a repeat key walk rather than stick.
		{name: "walks on", term: "svchost", after: 1, want: 3, found: true},
		// And wraps, so the last match is not a dead end.
		{name: "wraps", term: "svchost", after: 3, want: 1, found: true},
		{name: "case insensitive", term: "SVChost", after: -1, want: 1, found: true},
		{name: "by pid", term: "3", after: -1, want: 2, found: true},
		{name: "no match", term: "nothing", after: -1, found: false},
		// An empty term matches nothing rather than everything: an empty search box should
		// leave the cursor alone, not move it to row one.
		{name: "empty", term: "", after: 2, found: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, found := findTopMatch(rows, test.term, test.after)
			if found != test.found {
				t.Fatalf("findTopMatch(%q, %d) found = %v, want %v",
					test.term, test.after, found, test.found)
			}
			if found && got != test.want {
				t.Fatalf("findTopMatch(%q, %d) = %d, want %d",
					test.term, test.after, got, test.want)
			}
		})
	}
	// Nothing to search is not a crash.
	if _, found := findTopMatch(nil, "anything", -1); found {
		t.Fatal("found a match in no rows")
	}
}

// F3 is two keys in one, as it is in htop: ask for a term, then walk to the next match.
func TestTopModel_searchNextOnlyOnceThereIsSomethingToSearchFor(t *testing.T) {
	model := newTopModel(mustColumns(t))

	// When -- F3 with nothing searched for yet
	if got := model.applyKey("F3"); got != topActionSearchPrompt {
		t.Fatalf("F3 with no term asked for %v, want a prompt", got)
	}

	// And once there is a term
	model.Search = "chrome"
	if got := model.applyKey("F3"); got != topActionSearchNext {
		t.Fatalf("F3 with a term asked for %v, want the next match", got)
	}
	// n is the same thing, and / always asks again -- a new search rather than a repeat.
	if got := model.applyKey("n"); got != topActionSearchNext {
		t.Fatalf("n asked for %v, want the next match", got)
	}
	if got := model.applyKey("/"); got != topActionSearchPrompt {
		t.Fatalf("/ asked for %v, want a prompt", got)
	}
}

// The filter and the search must agree about what a match is, or the same word typed into each
// finds different processes.
func TestRowMatches_isSharedByTheFilterAndTheSearch(t *testing.T) {
	row := topRow{
		Process: proc.Process{PID: 4321, Name: "svchost.exe"},
		Details: proc.Details{CommandLine: `C:\Windows\system32\svchost.exe -k netsvcs`},
	}
	for _, term := range []string{"svchost", "SVCHOST", "netsvcs", "4321"} {
		model := newTopModel(mustColumns(t))
		model.Filter = term
		filtered := model.matches(row)
		_, searched := findTopMatch([]topRow{row}, term, -1)
		if !filtered || !searched {
			t.Fatalf("%q: filter matched %v but search matched %v", term, filtered, searched)
		}
	}
}
