package runtime

import "strings"

// logicalLines and the two ends of each logical line: where it begins, and how it is cut
// into the lines the parser reads once it is complete. The scanning between is
// syntax_scan.go's.

// The source must already have been through normalizeLineEndings.
func logicalLines(source string) ([]string, error) {
	lines, _, _, err := numberedLogicalLines(source, false)
	return lines, err
}

// numberedLogicalLines is logicalLines with, for each line, the index of the physical
// line its text starts on, and where in its text a later one begins (breaksWithin). waits is
// a session's input, which goes on past its text.
func numberedLogicalLines(source string, waits bool) ([]string, []int, [][]int, error) {
	physical := strings.Split(source, "\n")
	if len(physical) > 0 && physical[len(physical)-1] == "" {
		physical = physical[:len(physical)-1]
	}
	scanner := syntaxScanner{}
	for index, line := range physical {
		scanner.beginPhysicalLine(index)
		scanner.scanLine(line)
		scanner.finishPhysicalLine(line)
	}
	// A `\` that ends the input ends it there, as both references read one: before the last
	// newline it joins nothing, and the command ends; with none after it, it is a backslash of
	// its own, so `eval 'echo ok\'` says ok\. It was a line waiting for one that never came,
	// and the script was refused. A session's input goes on, and waits for the line.
	if last := len(physical) - 1; scanner.continued && !waits && last >= 0 && strings.HasSuffix(physical[last], `\`) {
		scanner.continued = false
		if !strings.HasSuffix(source, "\n") {
			scanner.logical.WriteString(`\\`)
		}
		scanner.finishPhysicalLine("")
	}
	if err := scanner.incompleteError(); err != nil {
		return scanner.lines, scanner.starts, scanner.lineBreaks, err
	}
	scanner.flushLogicalLine()
	if scanner.syntaxErr != nil {
		return scanner.lines, scanner.starts, scanner.lineBreaks, scanner.syntaxErr
	}
	return scanner.lines, scanner.starts, scanner.lineBreaks, nil
}

func (scanner *syntaxScanner) beginPhysicalLine(index int) {
	scanner.joined, scanner.continued = scanner.continued, false
	if scanner.logical.Len() == 0 {
		scanner.logicalStart, scanner.breaks = index, scanner.breaks[:0]
		return
	}
	scanner.breaks = append(scanner.breaks, scanner.logical.Len())
}

// segmentStart is the physical line the text at offset in the logical line starts on.
func (scanner *syntaxScanner) segmentStart(offset int) int {
	start := scanner.logicalStart
	for _, at := range scanner.breaks {
		if at <= offset {
			start++
		}
	}
	return start
}

func (scanner *syntaxScanner) flushLogicalLine() {
	if scanner.syntaxErr != nil {
		return
	}
	text := scanner.logical.String()
	segments, err := splitSequentialSegments(text)
	scanner.logical.Reset()
	if err != nil {
		scanner.syntaxErr = err
		return
	}
	// Each segment is a piece of text, in order, so finding it from where the last one
	// ended gives its offset, and the offset gives the physical line it starts on.
	searched := 0
	for _, segment := range segments {
		offset := searched
		if found := strings.Index(text[searched:], segment); found >= 0 {
			offset += found
			searched = offset + len(segment)
		}
		offset += len(segment) - len(strings.TrimLeft(segment, logicalLineCutset))
		if normalized := trimLogicalSegment(segment); normalized != "" {
			cursor := 0
			for _, line := range splitLeadingReservedWord(normalized) {
				at := offset + cursor
				if found := strings.Index(normalized[cursor:], line); found >= 0 {
					at, cursor = offset+cursor+found, cursor+found+len(line)
				}
				scanner.lines = append(scanner.lines, line)
				scanner.starts = append(scanner.starts, scanner.segmentStart(at))
				scanner.lineBreaks = append(scanner.lineBreaks, scanner.breaksWithin(at, len(line)))
			}
		}
	}
}

// trimLogicalSegment is a segment without the blanks around it -- all but one a backslash
// escapes at its end, which is part of the last word, as in busybox-w32 and bash: `echo a\ `
// prints `a ` and a space. Trimmed with the rest, it left the backslash last on the line, and
// the script was refused as ending in one.
func trimLogicalSegment(segment string) string {
	left := strings.TrimLeft(segment, logicalLineCutset)
	trimmed := strings.TrimRight(left, logicalLineCutset)
	backslashes := len(trimmed) - len(strings.TrimRight(trimmed, `\`))
	if backslashes%2 == 1 && len(trimmed) < len(left) && left[len(trimmed)] != '\n' {
		return left[:len(trimmed)+1]
	}
	return trimmed
}
