package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
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
		options, operands, err := parseAppletOptions(ctx, args, "uqiwBNsrabdTt", "ULS")
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
			ignoreSpace:        options.has('b'),
			ignoreBlank:        options.has('B'),
			treatAbsentAsEmpty: options.has('N'),
			labels:             options.all('L'),
			initialTab:         options.has('T'),
			expandTabs:         options.has('t'),
			text:               options.has('a'),
			minimal:            options.has('d'),
			recursive:          options.has('r'),
			start:              options.value('S'),
		}
		return request.diffOperands(ctx, stdin, stdout)
	}}
}

type diffRequest struct {
	left, right        string
	context            int
	brief              bool
	same               bool
	ignoreCase         bool
	ignoreAll          bool
	ignoreSpace        bool
	ignoreBlank        bool
	treatAbsentAsEmpty bool
	// labels are -L's, and initialTab and expandTabs -T and -t; see diff_unified.go. text
	// is -a, which diffs files that hold a NUL as text.
	labels                       []string
	initialTab, expandTabs, text bool
	// minimal is -d, under which stone tries every line of a class rather than stopping
	// after max(256, √n); see diff_stone.go. It was taken and ignored.
	minimal bool
	// recursive is -r and start -S; absent is, for -N in a directory, which side has no such
	// file, which is read as empty and named /dev/null. See diff_dir.go.
	recursive bool
	start     string
	absent    [2]bool
}

func (r diffRequest) run(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	var left, right []string
	var err error
	if !r.absent[0] {
		if left, err = readDiffLines(ctx, r.left, stdin, r.treatAbsentAsEmpty); err != nil {
			return err
		}
	}
	if !r.absent[1] {
		if right, err = readDiffLines(ctx, r.right, stdin, r.treatAbsentAsEmpty); err != nil {
			return err
		}
	}
	// Two files that are not the same and hold a NUL are binary, and that they differ is all
	// that is said, as busybox says it, but under -a. Their lines were diffed.
	if !r.text && (diffBinary(left) || diffBinary(right)) && !slices.Equal(left, right) {
		if _, err := fmt.Fprintf(stdout, "Files %s and %s differ\n", r.left, r.right); err != nil {
			return err
		}
		return ErrExitFalse
	}
	// stone keeps a line's place in 32 bits, as busybox's keeps it in an int.
	if len(left) > diffMaxLines || len(right) > diffMaxLines {
		return ExitStatusMessage(2, errors.New("too many lines"))
	}
	edits := r.diffLines(left, right)
	hunks := groupDiffHunks(edits, r.context)
	if r.ignoreBlank {
		hunks = slices.DeleteFunc(hunks, func(hunk diffHunk) bool { return onlyEmptyLinesChange(edits[hunk.from:hunk.to]) })
	}
	if len(hunks) == 0 {
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
	if err := r.writeUnified(stdout, edits, hunks); err != nil {
		return err
	}
	return ErrExitFalse
}

// diffBinary is whether a file's lines hold a NUL.
func diffBinary(lines []string) bool {
	for _, line := range lines {
		if strings.IndexByte(line, 0) >= 0 {
			return true
		}
	}
	return false
}

// diffSpace is C's isspace.
func diffSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}

// onlyEmptyLinesChange is whether every line a hunk adds or removes is empty, the hunk -B
// passes over, as busybox's does; it was taken and ignored. A line of white space is not
// empty.
func onlyEmptyLinesChange(edits []diffEdit) bool {
	for _, edit := range edits {
		if edit.kind != editKeep && edit.text != "\n" {
			return false
		}
	}
	return true
}

// readDiffLines is a FILE's lines, each with the newline that ends it, as busybox's read_token
// reads it into the line: a last line with none is another line than the same text with one, and
// the files differ, which they did not. A CRLF ends a line as an LF does.
func readDiffLines(ctx context.Context, path string, stdin io.Reader, absentIsEmpty bool) ([]string, error) {
	var lines []string
	// busybox's diff stats each FILE first, and names one that is not there as it cannot stat
	// it: `diff: can't stat 'FILE'`.
	err := eachTextInputNaming(ctx, []string{path}, stdin, func(reader io.Reader) error {
		return eachLine(reader, func(line, ending string) error {
			if ending != "" {
				line += "\n"
			}
			lines = append(lines, line)
			return nil
		})
	}, cannotStat)
	if err != nil {
		if absentIsEmpty {
			// -N: a missing file is an empty one, which is how a diff shows a
			// whole new file rather than failing on it.
			return nil, nil
		}
		// Trouble is 2, as busybox's diff has it, apart from 1 for files that differ. A missing
		// operand answered 1, and a script could not tell it from a difference.
		return nil, ExitStatusMessage(2, err)
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
	// leftLine and rightLine are one-based positions. Where the line does not exist on a side,
	// they are the line of that side it follows, 0 before the first: what a hunk with no line
	// of that side is numbered by, as `@@ -2,0 +3,5 @@` under -U0 says five lines were added
	// after the left's second.
	leftLine, rightLine int
}

// diffLines is the edit script, found as busybox's diff finds it; see diff_stone.go. What -i,
// -w and -b ignore is in each line's key, which is how they change what "the same line" means
// rather than how the lines are compared: -w passes over every white space character, and -b
// reads a run of them as one space, the newline that ends the line among them, so white space
// at a line's end is no difference and a run at its start is one. It was a table of every line
// of one file against every line of the other, which a diff of two large files never finished
// filling.
func (r diffRequest) diffLines(left, right []string) []diffEdit {
	var keys [2][]string
	var hashes [2][]int
	for side, lines := range [2][]string{left, right} {
		keys[side], hashes[side] = make([]string, len(lines)), make([]int, len(lines))
		for index, line := range lines {
			keys[side][index], hashes[side][index] = r.diffKey(line)
		}
	}
	matches, _ := diffMatches(keys, hashes, r.minimal)
	return diffEditsOf(matches, left, right)
}
