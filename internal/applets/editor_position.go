package applets

import (
	"fmt"
	"strconv"
	"strings"
)

// `nano +42 file`, which opens at line 42: what a compiler's `file:42:` and `grep -n` hand
// someone to type. It was taken for a second file and refused, "only one file at a time; there
// are no multiple buffers". nano's form is +LINE, +LINE,COLUMN or +,COLUMN, both counted from
// 1, in front of the file; a line past the end is the last, as nano has it. nano's +/STRING and
// +?STRING, which open at a search, are refused by name rather than taken for a file.

// editorPosition is where the editor opens, line and column from 1; zero is the start.
type editorPosition struct {
	line, column int
}

// editorOperands takes the +LINE[,COLUMN] operand out of operands, and answers where to open
// and the rest.
func editorOperands(operands []string) (editorPosition, []string, error) {
	var position editorPosition
	var rest []string
	for _, operand := range operands {
		if !strings.HasPrefix(operand, "+") || operand == "+" {
			rest = append(rest, operand)
			continue
		}
		parsed, err := parseEditorPosition(operand[1:])
		if err != nil {
			return editorPosition{}, nil, err
		}
		position = parsed
	}
	return position, rest, nil
}

func parseEditorPosition(text string) (editorPosition, error) {
	if strings.HasPrefix(text, "/") || strings.HasPrefix(text, "?") {
		return editorPosition{}, fmt.Errorf("+%s: opening at a search is not supported; +LINE,COLUMN is", text)
	}
	lineText, columnText, _ := strings.Cut(text, ",")
	var position editorPosition
	for _, part := range []struct {
		text string
		into *int
	}{{lineText, &position.line}, {columnText, &position.column}} {
		if part.text == "" {
			continue
		}
		number, err := strconv.Atoi(part.text)
		if err != nil || number < 1 {
			return editorPosition{}, fmt.Errorf("+%s: not a line and column; +LINE,COLUMN counts from 1", text)
		}
		*part.into = number
	}
	return position, nil
}

// moveToPosition puts the cursor where the editor was asked to open: the line, the last if the
// text is shorter, and the column on it, the end if the line is shorter. A column is a
// character, which the cursor offset counts in bytes; see moveToLine.
func (v *editorView) moveToPosition(position editorPosition) {
	if position.line == 0 && position.column == 0 {
		return
	}
	lines := editorLines(v.area.GetText())
	row := max(position.line, 1) - 1
	if row >= len(lines) {
		row = max(0, len(lines)-1)
	}
	offset := 0
	for index := range row {
		offset += len(lines[index]) + 1
	}
	if position.column > 1 && row < len(lines) {
		line := []rune(lines[row])
		column := min(position.column-1, len(line))
		offset += len(string(line[:column]))
	}
	v.area.Select(offset, offset)
}
