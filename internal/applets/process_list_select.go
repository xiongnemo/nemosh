package applets

import (
	"cmp"
	"slices"
	"strings"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// Which processes ps prints, and in what order.

// selected is the rows the options choose, by pid. Every process but the idle one, whose pid is
// 0 and whose CPU time is the machine's idleness: no ps lists a pid 0, and `--sort=-time` would put
// it first.
func (o psOptions) selected(table *psTable) []*psRow {
	rows := make([]*psRow, 0, len(table.processes))
	for _, process := range table.processes {
		if process.PID == 0 {
			continue
		}
		if row := (&psRow{process: process, table: table}); o.selects(row) {
			rows = append(rows, row)
		}
	}
	slices.SortFunc(rows, func(a, b *psRow) int { return cmp.Compare(a.process.PID, b.process.PID) })
	return rows
}

// selects is procps' rule: the selections are a union, -N turns it over, and BSD's r keeps only
// what is running.
func (o psOptions) selects(row *psRow) bool {
	chosen := o.chooses(row)
	if o.negate {
		chosen = !chosen
	}
	return chosen && (!o.running || row.process.State == proc.StateRunning)
}

func (o psOptions) chooses(row *psRow) bool {
	switch {
	case o.all:
		return true
	case len(o.pids)+len(o.ppids)+len(o.users)+len(o.commands) > 0:
		return slices.Contains(o.pids, row.process.PID) || slices.Contains(o.ppids, row.process.PPID) ||
			o.namesCommand(row.process.Name) || o.namesUser(row)
	case o.bsd:
		// BSD's form without `a` is this account's own processes.
		return row.table.self != "" && strings.EqualFold(row.user(), row.table.self)
	}
	return true
}

// namesCommand matches -C as pgrep matches a name: without regard to case, which Windows has none
// of, and with or without the executable's suffix, since `-C notepad` is what anyone types.
func (o psOptions) namesCommand(name string) bool {
	for _, command := range o.commands {
		if strings.EqualFold(name, command) || strings.EqualFold(trimExecutableSuffix(name), command) {
			return true
		}
	}
	return false
}

func (o psOptions) namesUser(row *psRow) bool {
	if len(o.users) == 0 {
		return false
	}
	user := row.detail().User
	return user != "" && slices.ContainsFunc(o.users, func(name string) bool { return strings.EqualFold(name, user) })
}

// order sorts the rows by the keys asked for, ties left in pid order, and under --forest or -H
// arranges them as a tree, the siblings sorted.
func (o psOptions) order(rows []*psRow) []*psRow {
	compare := o.comparison()
	if o.hierarchy == psFlat {
		if compare != nil {
			slices.SortStableFunc(rows, compare)
		}
		return rows
	}
	byPID := make(map[int]*psRow, len(rows))
	processes := make([]proc.Process, 0, len(rows))
	for _, row := range rows {
		byPID[row.process.PID] = row
		processes = append(processes, row.process)
	}
	var less func(a, b proc.Process) bool
	if compare != nil {
		less = func(a, b proc.Process) bool { return compare(byPID[a.PID], byPID[b.PID]) < 0 }
	}
	ordered := make([]*psRow, 0, len(rows))
	var walk func(nodes []*proc.Node, lead string)
	walk = func(nodes []*proc.Node, lead string) {
		for index, node := range nodes {
			row := byPID[node.Process.PID]
			switch {
			case node.Depth == 0:
			case o.hierarchy == psForest:
				row.prefix = lead + ` \_ `
			default:
				row.prefix = strings.Repeat("  ", node.Depth)
			}
			ordered = append(ordered, row)
			next := lead
			if node.Depth > 0 && index < len(nodes)-1 {
				next += " |  "
			} else if node.Depth > 0 {
				next += "    "
			}
			walk(node.Children, next)
		}
	}
	walk(proc.Tree(processes, less), "")
	return ordered
}

// comparison is the sort keys in turn, each reversed if it was asked for with a `-`.
func (o psOptions) comparison() func(a, b *psRow) int {
	if len(o.sort) == 0 {
		return nil
	}
	return func(a, b *psRow) int {
		for _, key := range o.sort {
			order := key.field.compare(a, b)
			if key.descending {
				order = -order
			}
			if order != 0 {
				return order
			}
		}
		return 0
	}
}
