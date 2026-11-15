package runtime

import (
	"strconv"
	"strings"
	"time"
)

// The history file's format, which the session that appends to it and `history -a -w -r -n`
// share, so the two cannot read one file two ways.

// HistoryRecord is one entry of a history file: the command, and when it was run, or the zero
// time when the file did not say.
type HistoryRecord struct {
	Line string
	When time.Time
}

// ParseHistoryFile reads a history file into its entries, in bash's format. A line that is a #
// and digits is the time of the entry after it, which bash writes under HISTTIMEFORMAT; in a
// file that has them, the lines up to the next one are one entry, which is how a command of
// several lines is kept whole. In a file with none, each line is an entry -- busybox's format,
// and this shell's without HISTTIMEFORMAT -- and a blank line is no entry.
func ParseHistoryFile(text string) []HistoryRecord {
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "\n")
	stamped := false
	for _, line := range lines {
		if _, ok := historyStamp(line); ok {
			stamped = true
			break
		}
	}
	var records []HistoryRecord
	var current HistoryRecord
	open, started := false, false
	flush := func() {
		if open && strings.TrimSpace(current.Line) != "" {
			records = append(records, current)
		}
	}
	for _, line := range lines {
		if when, ok := historyStamp(line); ok {
			flush()
			current, open, started = HistoryRecord{When: when}, true, false
			continue
		}
		if !stamped || !open {
			// A file with no times, or the lines of one before its first: one entry each.
			if strings.TrimSpace(line) != "" {
				records = append(records, HistoryRecord{Line: line})
			}
			continue
		}
		if started {
			current.Line += "\n" + line
		} else {
			current.Line, started = line, true
		}
	}
	flush()
	return records
}

// historyStamp reads a line that is a # and digits as the time it names.
func historyStamp(line string) (time.Time, bool) {
	digits, found := strings.CutPrefix(line, "#")
	if !found || digits == "" || strings.TrimLeft(digits, "0123456789") != "" {
		return time.Time{}, false
	}
	seconds, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(seconds, 0), true
}

// FormatHistoryRecord is an entry as the file keeps it: under HISTTIMEFORMAT its time on the
// line before, as bash writes it, and an entry with no time of its own given the present one.
func FormatHistoryRecord(record HistoryRecord, stamped bool) string {
	if !stamped {
		return record.Line + "\n"
	}
	when := record.When
	if when.IsZero() {
		when = time.Now()
	}
	return "#" + strconv.FormatInt(when.Unix(), 10) + "\n" + record.Line + "\n"
}
