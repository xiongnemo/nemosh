package proc

import "sort"

// The process tree, and why Windows makes it harder than Linux does.
//
// Linux reparents an orphan onto init, so every process has a living parent and the tree is a
// tree. Windows does neither: when a parent exits, its children keep the parent id they were
// born with, and that id goes back into the pool and is handed to something else. So a naive
// tree built from parent ids on Windows produces two wrong shapes -- a process adopted by a
// stranger that happens to have inherited the number, and a cycle when two processes end up
// pointing at each other's ids.
//
// The fix is that a parent must be *older than its child*. A process cannot have been started by
// something that started after it, so a candidate parent whose creation time is later than the
// child's is not the parent -- it is the reuse of a number. That single test removes both the
// wrong adoptions and, because "older than" is a strict order, every possible cycle.

// Node is one process in the tree, with its children beneath it.
type Node struct {
	Process  Process
	Depth    int
	Children []*Node
}

// Tree arranges processes by parentage, returning the roots.
//
// Ordering is by the caller's comparison, applied among siblings, so a tree sorted by CPU shows
// the busiest branch first without breaking parentage.
func Tree(processes []Process, less func(a, b Process) bool) []*Node {
	nodes := make(map[int]*Node, len(processes))
	for _, process := range processes {
		nodes[process.PID] = &Node{Process: process}
	}
	var roots []*Node
	for _, process := range processes {
		node := nodes[process.PID]
		parent, ok := nodes[process.PPID]
		if !ok || !isParent(parent.Process, process) {
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	sortNodes(roots, less)
	assignDepth(roots, 0)
	return roots
}

// isParent reports whether candidate can really be the parent of child.
//
// The creation-time test is the whole guard. Without it, `explorer.exe` regularly appears as the
// child of whatever inherited the id of the long-dead installer that launched it.
func isParent(candidate, child Process) bool {
	if candidate.PID == child.PID {
		// A process cannot be its own parent, and PID 0 lists itself as its parent.
		return false
	}
	if candidate.Created.IsZero() || child.Created.IsZero() {
		// No times to compare: accept the claim, because the alternative is a flat list.
		return true
	}
	return !candidate.Created.After(child.Created)
}

func sortNodes(nodes []*Node, less func(a, b Process) bool) {
	if less != nil {
		sort.SliceStable(nodes, func(i, j int) bool { return less(nodes[i].Process, nodes[j].Process) })
	}
	for _, node := range nodes {
		sortNodes(node.Children, less)
	}
}

func assignDepth(nodes []*Node, depth int) {
	for _, node := range nodes {
		node.Depth = depth
		assignDepth(node.Children, depth+1)
	}
}

// Flatten walks the tree in display order: a node, then everything beneath it.
//
// collapsed names the pids whose children are hidden, so a caller can fold a branch without
// rebuilding the tree.
func Flatten(roots []*Node, collapsed map[int]bool) []*Node {
	var rows []*Node
	var walk func(nodes []*Node)
	walk = func(nodes []*Node) {
		for _, node := range nodes {
			rows = append(rows, node)
			if !collapsed[node.Process.PID] {
				walk(node.Children)
			}
		}
	}
	walk(roots)
	return rows
}
