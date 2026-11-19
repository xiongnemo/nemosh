package applets

import (
	"bufio"
	"strings"

	"github.com/rivo/uniseg"
)

// column -t: the rows split as column.c splits them, laid out as libsmartcols lays out a table.

func writeColumnTable(out *bufio.Writer, lines []string, settings columnOptions) {
	var rows [][]string
	if len(settings.names) > 0 && !settings.noHeadings {
		rows = append(rows, settings.names)
	}
	for _, line := range lines {
		rows = append(rows, columnFields(line, settings))
	}
	var widths []int
	for _, row := range rows {
		for column, field := range row {
			if column >= len(widths) {
				widths = append(widths, 0)
			}
			widths[column] = max(widths[column], uniseg.StringWidth(field))
		}
	}
	for _, row := range rows {
		var line strings.Builder
		for column, field := range row {
			if column > 0 {
				line.WriteString(settings.output)
			}
			padding := strings.Repeat(" ", widths[column]-uniseg.StringWidth(field))
			switch {
			case settings.right[column]:
				line.WriteString(padding + field)
			case column == len(row)-1:
				line.WriteString(field)
			default:
				line.WriteString(field + padding)
			}
		}
		out.WriteString(line.String() + "\n")
	}
}

// columnFields is a line's fields: at runs of spaces and tabs, which wcstok merges, or at each of
// -s's characters, which local_wcstok does not; with -l the last of them is the rest of the line.
func columnFields(line string, settings columnOptions) []string {
	if line == "" {
		return nil
	}
	isSeparator := func(r rune) bool { return r == ' ' || r == '\t' }
	if settings.separators != "" {
		isSeparator = func(r rune) bool { return strings.ContainsRune(settings.separators, r) }
	}
	var fields []string
	rest := line
	for rest != "" {
		if settings.separators == "" {
			rest = strings.TrimLeftFunc(rest, isSeparator)
			if rest == "" {
				break
			}
		}
		if settings.limit > 0 && len(fields) == settings.limit-1 {
			fields = append(fields, rest)
			break
		}
		end := strings.IndexFunc(rest, isSeparator)
		if end < 0 {
			fields = append(fields, rest)
			break
		}
		fields = append(fields, rest[:end])
		_, width := firstRune(rest[end:])
		rest = rest[end+width:]
		if rest == "" && settings.separators != "" {
			// A separator at the end ends an empty last field.
			fields = append(fields, "")
		}
	}
	return fields
}

// firstRune is the first rune of text and its length in bytes.
func firstRune(text string) (rune, int) {
	for _, r := range text {
		return r, len(string(r))
	}
	return 0, 0
}
