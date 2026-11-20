package applets

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// :s/PATTERN/REPLACEMENT/FLAGS, busybox's: any character after the s is the delimiter, the pattern
// a basic regular expression as / takes one, an empty one the last search's, and \0 to \9 the
// match and its groups. vim's where they differ: & is the match and \& an ampersand, \r breaks
// the line there, the flags are g i and I -- busybox read only the first character, and its g
// matched ^ again wherever it resumed -- and a missing replacement is an empty one. The whole
// range is one change to undo.

// substitute is :s over lines first to last.
func (e *viEditor) substitute(first, last int, argument string) {
	delimiter, size := utf8.DecodeRuneInString(argument)
	if argument == "" || delimiter == '\\' || isViWord(delimiter) || delimiter == ' ' {
		e.say(true, ":s expression missing delimiters")
		return
	}
	pattern, rest := splitSearch(argument[size:], delimiter)
	replacement, flags := splitSearch(rest, delimiter)
	global, ignoreCase := false, e.ignoreCase
	for _, flag := range flags {
		switch flag {
		case 'g':
			global = true
		case 'i':
			ignoreCase = true
		case 'I':
			ignoreCase = false
		case 'c':
			e.say(true, "the c flag of :s is not implemented yet")
			return
		default:
			e.say(true, "Trailing characters: %s", flags)
			return
		}
	}
	if pattern != "" {
		e.lastPattern, e.searchCompiled = pattern, nil
	}
	if e.lastPattern == "" {
		e.say(true, "No previous search")
		return
	}
	translated, err := translateBasicRegex(e.lastPattern)
	if err == nil && ignoreCase {
		translated = "(?i)" + translated
	}
	var re *regexp.Regexp
	if err == nil {
		re, err = regexp.Compile(translated)
	}
	if err != nil {
		e.say(true, ":s bad search pattern")
		return
	}
	count, changed, row := 0, 0, last-1
	for index := first - 1; index <= row; index++ {
		text, found := replaceMatches(re, string(e.lines[index]), replacement, global)
		if found == 0 {
			continue
		}
		pieces := strings.Split(text, "\n")
		lines := make([][]rune, len(pieces))
		for at, piece := range pieces {
			lines[at] = []rune(piece)
		}
		e.replaceLines(index, index+1, lines)
		index += len(lines) - 1
		row += len(lines) - 1
		count, changed = count+found, changed+1
		e.row = index
	}
	if count == 0 {
		e.say(true, "No match")
		return
	}
	e.col = e.firstNonBlank(e.row)
	if count > 1 {
		e.say(false, "%d substitutions on %d lines", count, changed)
	}
}

// replaceMatches is text with the first match of re, or each, replaced, and how many there were.
func replaceMatches(re *regexp.Regexp, text, replacement string, global bool) (string, int) {
	limit := 1
	if global {
		limit = -1
	}
	matches := re.FindAllStringSubmatchIndex(text, limit)
	var out strings.Builder
	at := 0
	for _, match := range matches {
		out.WriteString(text[at:match[0]])
		expandReplacement(&out, replacement, text, match)
		at = match[1]
	}
	out.WriteString(text[at:])
	return out.String(), len(matches)
}

// expandReplacement writes replacement for one match: & and \0 the match, \1 to \9 its groups,
// \r and \n a line break, and a backslash before anything else that character.
func expandReplacement(out *strings.Builder, replacement, text string, match []int) {
	group := func(number int) {
		if 2*number+1 < len(match) && match[2*number] >= 0 {
			out.WriteString(text[match[2*number]:match[2*number+1]])
		}
	}
	for index := 0; index < len(replacement); index++ {
		switch char := replacement[index]; {
		case char == '&':
			group(0)
		case char == '\\' && index+1 < len(replacement):
			index++
			switch next := replacement[index]; {
			case next >= '0' && next <= '9':
				group(int(next - '0'))
			case next == 'r' || next == 'n':
				out.WriteByte('\n')
			default:
				out.WriteByte(next)
			}
		default:
			out.WriteByte(char)
		}
	}
}
