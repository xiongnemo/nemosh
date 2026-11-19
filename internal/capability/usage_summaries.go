package capability

import (
	"sort"
	"strings"
)

// Summary is a command's name and the line that says what it does, which man -k and whatis list.
type Summary struct{ Name, Line string }

// Summaries is every command with help, the applets' and the builtins', in name order, each line
// as man-db's descriptions read: lower case to begin and no full stop.
func Summaries() []Summary {
	var summaries []Summary
	add := func(usage map[string]Usage) {
		for name, entry := range usage {
			line := strings.TrimSuffix(entry.Summary, ".")
			if line != "" {
				line = strings.ToLower(line[:1]) + line[1:]
			}
			summaries = append(summaries, Summary{Name: name, Line: line})
		}
	}
	add(usageText())
	add(builtinUsage())
	sort.Slice(summaries, func(a, b int) bool { return summaries[a].Name < summaries[b].Name })
	return summaries
}
