package applets

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Searching, busybox's: / and ? a POSIX basic regular expression, as busybox's GNU re_search
// reads one with RE_SYNTAX_POSIX_BASIC, translated as sed's is; n and N again, the same way and
// the other; a match line by line, wrapping past either end with busybox's words. vim's where
// they differ: an unescaped / ends the pattern (`/b/`), ? finds a match just before the cursor,
// and * and # look for the word under the cursor.

// searchFor is / or ?, pattern what was typed after it.
func (e *viEditor) searchFor(prompt rune, typed string) bool {
	pattern, offset := splitSearch(typed, prompt)
	if offset != "" {
		e.say(true, "search offsets are not implemented")
		e.failed = true
		return false
	}
	if pattern != "" {
		e.lastPattern, e.searchCompiled = pattern, nil
	}
	e.lastDirection = 1
	if prompt == '?' {
		e.lastDirection = -1
	}
	return e.searchAgain(e.lastDirection)
}

// splitSearch parts what follows / at its first unescaped /, vim's end of a pattern.
func splitSearch(typed string, delimiter rune) (string, string) {
	for index := 0; index < len(typed); index++ {
		switch {
		case typed[index] == '\\':
			index++
		case rune(typed[index]) == delimiter:
			return typed[:index], typed[index+1:]
		}
	}
	return typed, ""
}

// searchAgain moves the cursor to the count'th match in direction dir.
func (e *viEditor) searchAgain(dir int) bool {
	if e.lastPattern == "" {
		e.say(true, "No previous search")
		e.failed = true
		return false
	}
	re, err := e.compiledSearch()
	if err != nil {
		e.say(true, "bad search pattern '%s': %v", e.lastPattern, err)
		e.failed = true
		return false
	}
	p := e.cursor()
	for range e.times() {
		next, wrapped, ok := e.findMatch(re, p, dir)
		if !ok {
			e.say(true, "Pattern not found")
			e.failed = true
			return false
		}
		switch {
		case wrapped && dir > 0:
			e.say(true, "search hit BOTTOM, continuing at TOP")
		case wrapped:
			e.say(true, "search hit TOP, continuing at BOTTOM")
		}
		p = next
	}
	e.setCursor(p)
	return true
}

func (e *viEditor) compiledSearch() (*regexp.Regexp, error) {
	if e.searchCompiled != nil && e.searchIgnoringCase == e.ignoreCase {
		return e.searchCompiled, nil
	}
	translated, err := translateBasicRegex(e.lastPattern)
	if err != nil {
		return nil, err
	}
	if e.ignoreCase {
		translated = "(?i)" + translated
	}
	re, err := regexp.Compile(translated)
	if err == nil {
		e.searchCompiled, e.searchIgnoringCase = re, e.ignoreCase
	}
	return re, err
}

// findMatch is the first match after p going forward, or the last before it going back, and
// whether the search went past the buffer's end to find it.
func (e *viEditor) findMatch(re *regexp.Regexp, p viPos, dir int) (viPos, bool, bool) {
	count := len(e.lines)
	for step := 0; step <= count; step++ {
		row := ((p.row+dir*step)%count + count) % count
		wrapped := dir > 0 && row < p.row || dir < 0 && row > p.row || step == count
		starts := matchStarts(re, e.lines[row])
		if dir < 0 {
			for index := len(starts) - 1; index >= 0; index-- {
				if step > 0 || starts[index] < p.col {
					return viPos{row, starts[index]}, wrapped, true
				}
			}
			continue
		}
		for _, start := range starts {
			if step > 0 || start > p.col {
				return viPos{row, start}, wrapped, true
			}
		}
	}
	return p, false, false
}

// matchStarts is where re matches in line, as rune indexes.
func matchStarts(re *regexp.Regexp, line []rune) []int {
	text := string(line)
	var starts []int
	for _, match := range re.FindAllStringIndex(text, -1) {
		starts = append(starts, utf8.RuneCountInString(text[:match[0]]))
	}
	return starts
}

// searchWord is * and #, vim's: the word under the cursor, or the next on its line, as a whole
// word, forward or back.
func (e *viEditor) searchWord(key rune) bool {
	line, start := e.line(), e.col
	for start < len(line) && !isViWord(line[start]) {
		start++
	}
	if start >= len(line) {
		e.say(true, "No string under cursor")
		e.failed = true
		return false
	}
	for start > 0 && isViWord(line[start-1]) {
		start--
	}
	end := start
	for end < len(line) && isViWord(line[end]) {
		end++
	}
	e.lastPattern, e.searchCompiled = `\<`+string(line[start:end])+`\>`, nil
	e.lastDirection, e.col = 1, start
	if key == '#' {
		e.lastDirection = -1
	}
	return e.searchAgain(e.lastDirection)
}

// isSearch is a key whose motion is a search.
func isSearch(key rune) bool { return strings.ContainsRune("/?nN*#", key) }
