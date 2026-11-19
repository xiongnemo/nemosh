package applets

import (
	"bufio"
	"slices"
	"strconv"
	"strings"
)

// pstree's dump_tree, out_char's cutting of a line, and the widths a child is indented by.

type pstreeRun struct {
	out                                *bufio.Writer
	symbols                            pstreeSymbols
	limit                              int
	truncate, pids, arguments, compact bool
	// x is out_char's cur_x, the column written to; width and more are dump_tree's, by level.
	x     int
	width []int
	more  []bool
}

// put writes text as out_char does: past the line's limit nothing, but a + in the column after it.
func (r *pstreeRun) put(text string) int {
	written := 0
	for _, character := range text {
		r.x++
		written += pstreeWidth(string(character))
		switch {
		case !r.truncate || r.x <= r.limit:
			r.out.WriteRune(character)
		case r.x == r.limit+1:
			r.out.WriteByte('+')
		}
	}
	return written
}

func (r *pstreeRun) newline() {
	r.out.WriteByte('\n')
	r.x = 0
}

// dump is dump_tree: leaf says the node goes on its parent's line, last that it is the last child,
// rep how many identical siblings it stands for, closing how many ] its line owes.
func (r *pstreeRun) dump(node *pstreeNode, level, rep int, leaf, last bool, closing int) {
	for len(r.width) <= level {
		r.width, r.more = append(r.width, 0), append(r.more, false)
	}
	if !leaf {
		for depth := 0; depth < level; depth++ {
			r.put(strings.Repeat(" ", r.width[depth]+1))
			switch {
			case depth == level-1 && last:
				r.put(r.symbols.last)
			case depth == level-1:
				r.put(r.symbols.branch)
			case r.more[depth+1]:
				r.put(r.symbols.vertical)
			default:
				r.put(r.symbols.empty)
			}
		}
	}
	add := 0
	if rep >= 2 {
		add = r.put(strconv.Itoa(rep) + "*[")
	}
	nameWidth := r.put(node.name)
	offset := r.x
	if r.pids && !node.thread {
		r.put("(" + strconv.Itoa(node.pid) + ")")
	}
	if r.arguments && node.arguments != "" {
		r.put(" " + node.arguments)
	}
	if r.arguments || len(node.children) == 0 {
		r.put(strings.Repeat("]", closing))
		r.newline()
	}
	r.more[level] = !last
	children := slices.Clone(node.children)
	if r.arguments {
		// -a: a process a line, two columns in a level; only threads folded.
		r.width[level] = 1
		for index := 0; index < len(children); index++ {
			count := r.fold(&children, index, true)
			r.dump(children[index], level+1, count+1, false, index == len(children)-1, min(count, 1))
		}
		return
	}
	r.width[level] = nameWidth + r.x - offset + add
	if r.x >= r.limit && r.truncate {
		r.put(r.symbols.first + "+")
		r.newline()
		return
	}
	for index := 0; index < len(children); index++ {
		count := r.fold(&children, index, false)
		next := index < len(children)-1
		if index == 0 {
			if next {
				r.put(r.symbols.first)
			} else {
				r.put(r.symbols.single)
			}
		}
		extra := 0
		if count > 0 {
			extra = 1
		}
		r.dump(children[index], level+1, count+1, index == 0, !next, closing+extra)
	}
}

// fold takes out of children the later siblings identical to the one at index and answers how
// many: every identical sibling when folding, the threads alone under -a or -p, which leave
// processes apart.
func (r *pstreeRun) fold(children *[]*pstreeNode, index int, threadsOnly bool) int {
	node := (*children)[index]
	if !r.compact && !node.thread || threadsOnly && !node.thread {
		return 0
	}
	count := 0
	for scan := index + 1; scan < len(*children); {
		if pstreeEqual(node, (*children)[scan]) {
			*children = slices.Delete(*children, scan, scan+1)
			count++
			continue
		}
		scan++
	}
	return count
}
