package runtime

import "strings"

// logicalLines and the two ends of each logical line: where it begins, and how it is cut
// into the lines the parser reads once it is complete. The scanning between is
// syntax_scan.go's.

// The source must already have been through normalizeLineEndings.
func logicalLines(source string) ([]string, error) {
	lines, _, err := numberedLogicalLines(source)
	return lines, err
}

// numberedLogicalLines is logicalLines with, for each line, the index of the physical
// line its text starts on.
func numberedLogicalLines(source string) ([]string, []int, error) {
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
	if err := scanner.incompleteError(); err != nil {
		return scanner.lines, scanner.starts, err
	}
	scanner.flushLogicalLine()
	if scanner.syntaxErr != nil {
		return scanner.lines, scanner.starts, scanner.syntaxErr
	}
	return scanner.lines, scanner.starts, nil
}

func (scanner *syntaxScanner) beginPhysicalLine(index int) {
	scanner.continued = false
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
		if normalized := strings.Trim(segment, logicalLineCutset); normalized != "" {
			for _, line := range splitLeadingReservedWord(normalized) {
				scanner.lines = append(scanner.lines, line)
				scanner.starts = append(scanner.starts, scanner.segmentStart(offset))
			}
		}
	}
}
