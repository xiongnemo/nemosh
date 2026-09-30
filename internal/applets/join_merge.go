package applets

import (
	"bufio"
	"io"
	"strings"
)

// The merge that join.go's options drive: busybox's readfields, printfields and main loop.

// merge is busybox's main loop: of two sets the one with the smaller key is unpaired and read
// past, equal keys pair their sets and both are read past, and once one file ends what is left
// of the other is unpaired.
func (spec joinSpec) merge(out *bufio.Writer, files [2]*joinFile) error {
	for _, file := range files {
		if err := file.nextSet(); err != nil {
			return err
		}
	}
	left, right := files[0], files[1]
	for len(left.set) > 0 && len(right.set) > 0 {
		order := strings.Compare(left.key, right.key)
		var err error
		switch {
		case order == 0 && spec.paired:
			err = spec.print(out, left.key, left.set, right.set)
		case order < 0 && spec.unpaired[0]:
			err = spec.print(out, left.key, left.set, nil)
		case order > 0 && spec.unpaired[1]:
			err = spec.print(out, right.key, nil, right.set)
		}
		if err == nil && order <= 0 {
			err = left.nextSet()
		}
		if err == nil && order >= 0 {
			err = right.nextSet()
		}
		if err != nil {
			return err
		}
	}
	for index, file := range files {
		for spec.unpaired[index] && len(file.set) > 0 {
			var sets [2][][]string
			sets[index] = file.set
			if err := spec.print(out, file.key, sets[0], sets[1]); err != nil {
				return err
			}
			if err := file.nextSet(); err != nil {
				return err
			}
		}
	}
	return nil
}

// print writes a line for every line of lefts with every line of rights. A file without a set
// is one line that is not there, whose fields -o prints as -e's text.
func (spec joinSpec) print(out *bufio.Writer, key string, lefts, rights [][]string) error {
	separator := spec.separator
	if separator == 0 {
		separator = ' '
	}
	for _, left := range orMissingLine(lefts) {
		for _, right := range orMissingLine(rights) {
			for index, field := range spec.outputFields(key, [2][]string{left, right}) {
				if index > 0 {
					out.WriteByte(separator)
				}
				out.WriteString(field)
			}
			if err := out.WriteByte('\n'); err != nil {
				return err
			}
		}
	}
	return nil
}

func orMissingLine(set [][]string) [][]string {
	if set == nil {
		return [][]string{nil}
	}
	return set
}

// outputFields are one output line's fields: -o's list, or the join field and then each line's
// others. One that is empty, or that -o names and the line does not have, is -e's text.
func (spec joinSpec) outputFields(key string, lines [2][]string) []string {
	orEmpty := func(field string) string {
		if field == "" {
			return spec.empty
		}
		return field
	}
	if spec.listed {
		fields := make([]string, 0, len(spec.format))
		for _, entry := range spec.format {
			field := key
			if entry.file != 0 {
				field = ""
				if line := lines[entry.file-1]; entry.field < len(line) {
					field = line[entry.field]
				}
			}
			fields = append(fields, orEmpty(field))
		}
		return fields
	}
	fields := []string{orEmpty(key)}
	for index, line := range lines {
		for position, field := range line {
			if position != spec.fields[index] {
				fields = append(fields, orEmpty(field))
			}
		}
	}
	return fields
}

// joinFile is one of join's inputs, read a set at a time.
type joinFile struct {
	lines     *bufio.Scanner
	field     int
	separator byte
	// key and set are the current set: the join field its lines share, and the lines, split.
	// The set is empty once the input is.
	key string
	set [][]string
	// next is the line read past the set, which begins the one after it.
	next    []string
	hasNext bool
}

func newJoinFile(input io.Reader, field int, separator byte) *joinFile {
	lines := bufio.NewScanner(input)
	lines.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	lines.Split(scanLineWithEnding)
	return &joinFile{lines: lines, field: field, separator: separator}
}

// nextSet reads the lines in a row that share a key, as busybox's readfields does. A line without
// the join field has an empty key.
func (f *joinFile) nextSet() error {
	f.set = nil
	for {
		line := f.next
		if !f.hasNext {
			if !f.lines.Scan() {
				return f.lines.Err()
			}
			text, _ := splitLineEnding(f.lines.Text())
			line = splitJoinFields(text, f.separator)
		}
		f.hasNext = false
		key := ""
		if f.field < len(line) {
			key = line[f.field]
		}
		if len(f.set) > 0 && key != f.key {
			f.next, f.hasNext = line, true
			return nil
		}
		f.key = key
		f.set = append(f.set, line)
	}
}

// splitJoinFields is busybox's field_split: at each separator, so that two in a row have an
// empty field between them, or without one the runs of what is neither a blank nor a tab.
func splitJoinFields(line string, separator byte) []string {
	if separator != 0 {
		return strings.Split(line, string([]byte{separator}))
	}
	return strings.FieldsFunc(line, func(char rune) bool { return char == ' ' || char == '\t' })
}
