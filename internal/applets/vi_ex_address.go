package applets

import (
	"strconv"
	"strings"
)

// The addresses before an ex command, busybox's get_address and get_one_address: . $ N 'a /pat/
// ?pat?, each with +N and -N after it, a bare + or - being one; % for 1,$; a comma between two,
// or a semicolon, which makes the first the current line for the second. Of more than two the
// last two count. Lines are numbered from 1 here, as an ex command numbers them.

// exAddresses reads the addresses at the start of line: the first and last line, how many were
// given, and the rest of the line. A mark not set and a pattern not found say so, and ok is false.
func (e *viEditor) exAddresses(line string) (first, last, given int, rest string, ok bool) {
	current := e.row + 1
	first, last = current, current
	for {
		line = strings.TrimLeft(line, " \t")
		if strings.HasPrefix(line, "%") {
			first, last, given, line = 1, len(e.lines), 2, line[1:]
		} else {
			address, found, after, good := e.exOneAddress(line, current)
			if !good {
				return 0, 0, 0, "", false
			}
			separated := strings.HasPrefix(after, ",") || strings.HasPrefix(after, ";")
			if !found && !separated && given == 0 {
				return first, last, given, line, true
			}
			first, last, given, line = last, address, min(given+1, 2), after
		}
		line = strings.TrimLeft(line, " \t")
		switch {
		case strings.HasPrefix(line, ";"):
			current = last
		case !strings.HasPrefix(line, ","):
			if given == 1 {
				first = last
			}
			return first, last, given, line, true
		}
		line = line[1:]
	}
}

// exOneAddress reads one address, the current line when there is none: the line it names,
// whether one was there, and what follows it.
func (e *viEditor) exOneAddress(line string, current int) (int, bool, string, bool) {
	address, found := current, false
	switch {
	case strings.HasPrefix(line, "."):
		found, line = true, line[1:]
	case strings.HasPrefix(line, "$"):
		address, found, line = len(e.lines), true, line[1:]
	case strings.HasPrefix(line, "'") && len(line) > 1:
		place, set := e.markPlace(rune(line[1]))
		if !set {
			e.say(true, "Mark not set")
			return 0, false, "", false
		}
		address, found, line = place.row+1, true, line[2:]
	case strings.HasPrefix(line, "/") || strings.HasPrefix(line, "?"):
		pattern, after := splitSearch(line[1:], rune(line[0]))
		row, good := e.searchLine(pattern, line[0] == '/', current-1)
		if !good {
			return 0, false, "", false
		}
		address, found, line = row+1, true, after
	default:
		if digits := len(line) - len(strings.TrimLeft(line, "0123456789")); digits > 0 {
			address, _ = strconv.Atoi(line[:digits])
			found, line = true, line[digits:]
		}
	}
	for {
		line = strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "-") {
			return address, found, line, true
		}
		sign := 1
		if line[0] == '-' {
			sign = -1
		}
		line = line[1:]
		digits := len(line) - len(strings.TrimLeft(line, "0123456789"))
		offset := 1
		if digits > 0 {
			offset, _ = strconv.Atoi(line[:digits])
			line = line[digits:]
		}
		address, found = address+sign*offset, true
	}
}

// searchLine is the line a /pat/ or ?pat? address names: the next that matches after row going
// forward, or before it going back, past either end as busybox's char_search goes on.
func (e *viEditor) searchLine(pattern string, forward bool, row int) (int, bool) {
	if pattern != "" {
		e.lastPattern, e.searchCompiled = pattern, nil
	}
	if e.lastPattern == "" {
		e.say(true, "No previous search")
		return 0, false
	}
	re, err := e.compiledSearch()
	if err != nil {
		e.say(true, "bad search pattern '%s': %v", e.lastPattern, err)
		return 0, false
	}
	count, dir := len(e.lines), 1
	if !forward {
		dir = -1
	}
	for step := 1; step <= count; step++ {
		candidate := ((row+dir*step)%count + count) % count
		if re.MatchString(string(e.lines[candidate])) {
			return candidate, true
		}
	}
	e.say(true, "Pattern not found")
	return 0, false
}
