package proc_test

import (
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

func born(pid, ppid int, name string, after time.Duration) proc.Process {
	return proc.Process{PID: pid, PPID: ppid, Name: name, Created: epoch.Add(after)}
}

func byPID(a, b proc.Process) bool { return a.PID < b.PID }

// names flattens the tree into `depth:name` so a shape can be asserted in one line.
func names(roots []*proc.Node) []string {
	var out []string
	for _, node := range proc.Flatten(roots, nil) {
		prefix := ""
		for i := 0; i < node.Depth; i++ {
			prefix += "-"
		}
		out = append(out, prefix+node.Process.Name)
	}
	return out
}

func TestTree_ordinaryParentage(t *testing.T) {
	processes := []proc.Process{
		born(1, 0, "root", 0),
		born(2, 1, "child", time.Second),
		born(3, 2, "grandchild", 2*time.Second),
		born(4, 1, "sibling", 3*time.Second),
	}

	// When
	roots := proc.Tree(processes, byPID)

	// Then
	want := []string{"root", "-child", "--grandchild", "-sibling"}
	got := names(roots)
	if len(got) != len(want) {
		t.Fatalf("tree = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("tree = %v, want %v", got, want)
		}
	}
}

// The Windows hazard that a Linux tree never meets. A dead parent is not replaced by init, so its
// pid goes back into the pool; the process that inherits the number then appears to have adopted
// children it never started. The guard is that a parent must be older than its child.
func TestTree_refusesAParentYoungerThanItsChild(t *testing.T) {
	processes := []proc.Process{
		// pid 500 was recycled: this one started a minute after the child that claims it.
		born(500, 0, "stranger", time.Minute),
		born(600, 500, "orphan", 0),
	}

	// When
	roots := proc.Tree(processes, byPID)

	// Then -- both are roots. The orphan is not shown as the stranger's child.
	if len(roots) != 2 {
		t.Fatalf("tree = %v, want both as roots", names(roots))
	}
}

// Two processes each claiming the other would be a cycle, and a recursive walk over one would not
// return. The age rule makes that impossible rather than guarding against it separately: "older
// than" is a strict order, so a cycle cannot satisfy it in both directions.
func TestTree_cannotFormACycle(t *testing.T) {
	processes := []proc.Process{
		born(10, 11, "first", time.Second),
		born(11, 10, "second", 2*time.Second),
	}

	// When -- must terminate
	roots := proc.Tree(processes, byPID)

	// Then -- the older one parents the younger, and nothing loops.
	rows := names(roots)
	if len(rows) != 2 {
		t.Fatalf("tree = %v, want two rows", rows)
	}
	if rows[0] != "first" || rows[1] != "-second" {
		t.Fatalf("tree = %v, want the older as parent", rows)
	}
}

// PID 0 lists itself as its own parent on Windows, which would be a one-node cycle.
func TestTree_processThatIsItsOwnParent(t *testing.T) {
	processes := []proc.Process{born(0, 0, "Idle", 0), born(4, 0, "System", 0)}

	// When
	roots := proc.Tree(processes, byPID)

	// Then
	rows := names(roots)
	if len(rows) != 2 || rows[0] != "Idle" || rows[1] != "-System" {
		t.Fatalf("tree = %v, want Idle with System beneath it", rows)
	}
}

// A parent that is not in the list at all -- it exited between the sample and the tree -- leaves
// its children as roots rather than dropping them.
func TestTree_missingParentLeavesTheChildVisible(t *testing.T) {
	processes := []proc.Process{born(900, 899, "abandoned", time.Second)}

	// When
	roots := proc.Tree(processes, byPID)

	// Then
	if len(roots) != 1 || roots[0].Process.PID != 900 {
		t.Fatalf("tree = %v, want the child as a root", names(roots))
	}
}

// Sorting applies among siblings, so a tree ordered by anything still shows real parentage.
func TestTree_sortsSiblingsWithoutBreakingParentage(t *testing.T) {
	processes := []proc.Process{
		born(1, 0, "root", 0),
		born(5, 1, "later", time.Second),
		born(9, 1, "earlier", 2*time.Second),
	}
	descending := func(a, b proc.Process) bool { return a.PID > b.PID }

	// When
	roots := proc.Tree(processes, descending)

	// Then
	rows := names(roots)
	if rows[0] != "root" || rows[1] != "-earlier" || rows[2] != "-later" {
		t.Fatalf("tree = %v, want siblings in descending pid order under root", rows)
	}
}

func TestFlatten_collapsedBranchHidesItsChildren(t *testing.T) {
	processes := []proc.Process{
		born(1, 0, "root", 0),
		born(2, 1, "folded", time.Second),
		born(3, 2, "hidden", 2*time.Second),
	}
	roots := proc.Tree(processes, byPID)

	// When
	rows := proc.Flatten(roots, map[int]bool{2: true})

	// Then
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want the collapsed branch's child hidden", len(rows))
	}
}
