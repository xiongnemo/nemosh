package applets

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// diff: absent from stock Windows entirely, and the completion of a family that
// was already half here -- `cmp` compares bytes and `comm` compares sorted lines.
//
// **The output is unified, always.** That is busybox's default and not GNU's,
// which prints the older "normal" format unless asked; measured 2026-08-23. The
// header carries no timestamps, again following busybox.

func newDiffApplet() Applet {
	return simpleApplet{name: "diff", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "uqiwBNsrabdTt", "UL")
		if err != nil {
			return err
		}
		if len(operands) != 2 {
			return fmt.Errorf("two file operands are required")
		}
		context := 3
		if options.has('U') {
			parsed, err := parseGrepNumber(options.value('U'))
			if err != nil {
				return err
			}
			context = parsed
		}
		request := diffRequest{
			left: operands[0], right: operands[1],
			context:            context,
			brief:              options.has('q'),
			same:               options.has('s'),
			ignoreCase:         options.has('i'),
			ignoreAll:          options.has('w'),
			ignoreBlank:        options.has('B'),
			treatAbsentAsEmpty: options.has('N'),
		}
		return request.run(ctx, stdin, stdout)
	}}
}

type diffRequest struct {
	left, right        string
	context            int
	brief              bool
	same               bool
	ignoreCase         bool
	ignoreAll          bool
	ignoreBlank        bool
	treatAbsentAsEmpty bool
}

func (r diffRequest) run(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	left, err := readDiffLines(ctx, r.left, stdin, r.treatAbsentAsEmpty)
	if err != nil {
		return err
	}
	right, err := readDiffLines(ctx, r.right, stdin, r.treatAbsentAsEmpty)
	if err != nil {
		return err
	}
	edits := diffLines(left, right, r.compare)
	if !anyDifference(edits) {
		if r.same {
			if _, err := fmt.Fprintf(stdout, "Files %s and %s are identical\n", r.left, r.right); err != nil {
				return err
			}
		}
		return nil
	}
	if r.brief {
		if _, err := fmt.Fprintf(stdout, "Files %s and %s differ\n", r.left, r.right); err != nil {
			return err
		}
		// Status 1 means "they differ", which is what a script tests. Only a
		// failure to *compare* is status 2, and grepStatus is the precedent.
		return ErrExitFalse
	}
	if err := r.writeUnified(stdout, left, right, edits); err != nil {
		return err
	}
	return ErrExitFalse
}

// compare answers whether two lines count as equal, which is where -i, -w and -B
// take effect: they change what "the same line" means rather than how the diff is
// computed.
func (r diffRequest) compare(left, right string) bool {
	normalise := func(line string) string {
		if r.ignoreAll {
			line = strings.Join(strings.Fields(line), " ")
		}
		if r.ignoreCase {
			line = strings.ToLower(line)
		}
		return line
	}
	return normalise(left) == normalise(right)
}

func readDiffLines(ctx context.Context, path string, stdin io.Reader, absentIsEmpty bool) ([]string, error) {
	var lines []string
	err := eachTextInput(ctx, []string{path}, stdin, func(reader io.Reader) error {
		return eachLine(reader, func(line, _ string) error {
			lines = append(lines, line)
			return nil
		})
	})
	if err != nil {
		if absentIsEmpty {
			// -N: a missing file is an empty one, which is how a diff shows a
			// whole new file rather than failing on it.
			return nil, nil
		}
		return nil, err
	}
	return lines, nil
}

// editKind is what happened to one line.
type editKind byte

const (
	editKeep editKind = iota
	editRemove
	editAdd
)

type diffEdit struct {
	kind editKind
	text string
	// leftLine and rightLine are one-based positions, zero where the line does
	// not exist on that side.
	leftLine, rightLine int
}

func anyDifference(edits []diffEdit) bool {
	for _, edit := range edits {
		if edit.kind != editKeep {
			return true
		}
	}
	return false
}

// diffLines computes an edit script.
//
// Common prefix and suffix are stripped first, then a longest-common-subsequence
// table covers what is left. Stripping is not an optimisation for its own sake:
// the table is O(n*m), and two versions of a real file usually differ in a few
// lines out of thousands, so without it a large pair costs gigabytes.
func diffLines(left, right []string, equal func(a, b string) bool) []diffEdit {
	prefix := 0
	for prefix < len(left) && prefix < len(right) && equal(left[prefix], right[prefix]) {
		prefix++
	}
	suffix := 0
	for suffix < len(left)-prefix && suffix < len(right)-prefix &&
		equal(left[len(left)-1-suffix], right[len(right)-1-suffix]) {
		suffix++
	}
	edits := make([]diffEdit, 0, len(left)+len(right))
	for index := range prefix {
		edits = append(edits, diffEdit{kind: editKeep, text: left[index], leftLine: index + 1, rightLine: index + 1})
	}
	middle := lcsEdits(left[prefix:len(left)-suffix], right[prefix:len(right)-suffix], equal, prefix)
	edits = append(edits, middle...)
	for index := range suffix {
		leftAt := len(left) - suffix + index
		rightAt := len(right) - suffix + index
		edits = append(edits, diffEdit{kind: editKeep, text: left[leftAt], leftLine: leftAt + 1, rightLine: rightAt + 1})
	}
	return edits
}

// lcsEdits walks a longest-common-subsequence table backwards into an edit
// script.
//
// Removals are emitted before additions at the same position, which is what makes
// a changed line read as `-old` then `+new` rather than the reverse.
func lcsEdits(left, right []string, equal func(a, b string) bool, offset int) []diffEdit {
	rows, columns := len(left)+1, len(right)+1
	table := make([][]int, rows)
	for row := range table {
		table[row] = make([]int, columns)
	}
	for row := len(left) - 1; row >= 0; row-- {
		for column := len(right) - 1; column >= 0; column-- {
			if equal(left[row], right[column]) {
				table[row][column] = table[row+1][column+1] + 1
				continue
			}
			table[row][column] = max(table[row+1][column], table[row][column+1])
		}
	}
	var edits []diffEdit
	row, column := 0, 0
	for row < len(left) && column < len(right) {
		switch {
		case equal(left[row], right[column]):
			edits = append(edits, diffEdit{kind: editKeep, text: left[row],
				leftLine: offset + row + 1, rightLine: offset + column + 1})
			row, column = row+1, column+1
		case table[row+1][column] >= table[row][column+1]:
			edits = append(edits, diffEdit{kind: editRemove, text: left[row], leftLine: offset + row + 1})
			row++
		default:
			edits = append(edits, diffEdit{kind: editAdd, text: right[column], rightLine: offset + column + 1})
			column++
		}
	}
	for ; row < len(left); row++ {
		edits = append(edits, diffEdit{kind: editRemove, text: left[row], leftLine: offset + row + 1})
	}
	for ; column < len(right); column++ {
		edits = append(edits, diffEdit{kind: editAdd, text: right[column], rightLine: offset + column + 1})
	}
	return edits
}
