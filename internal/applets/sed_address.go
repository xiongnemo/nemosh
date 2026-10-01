package applets

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Addresses: which lines a command applies to.
//
// sed was an `s///` filter and nothing else, so `sed -n '5,10p'` and
// `sed '/x/d'` -- the two things people actually type -- did not run.
//
//	5        line five
//	$        the last line
//	/re/     every line matching
//	2,4      lines two to four
//	2,$      line two to the end
//	/a/,/b/  from a match on a to the next match on b
//	/a/,+2   a match on a and the two lines after it, busybox's and GNU's
//	5!       every line except five

// sedAddress selects lines. A zero value selects all of them, which is what an
// address-less command means.
type sedAddress struct {
	start sedEndpoint
	// end is the second half of a range, present only for `a,b`.
	end    sedEndpoint
	ranged bool
	// negated is the trailing `!`.
	negated bool
	// active tracks an open range between lines, which is why a program is
	// executed through a pointer: a range's state is per run, not per command.
	active bool
	// following is the N of `,+N`, which ends the range N lines after the line that
	// opened it, the line until; it was refused as no address after the comma.
	following, until int
	relative         bool
}

// sedEndpoint is one side of an address: a line number, the last line, or a
// pattern.
type sedEndpoint struct {
	line    int
	last    bool
	pattern *regexp.Regexp
}

func (e sedEndpoint) empty() bool { return e.line == 0 && !e.last && e.pattern == nil }

// matches reports whether an endpoint selects this line.
func (e sedEndpoint) matches(line string, number int, isLast bool) bool {
	switch {
	case e.pattern != nil:
		return e.pattern.MatchString(line)
	case e.last:
		return isLast
	}
	return e.line == number
}

// selects reports whether the command runs on this line, advancing any open
// range as it goes.
//
// A range that never sees its closing address runs to the end of the input,
// which is what both references do: `sed -n '/c/,/nosuch/p'` prints from the
// first c onward rather than nothing.
func (a *sedAddress) selects(line string, number int, isLast bool) bool {
	selected := a.selectsBeforeNegation(line, number, isLast)
	if a.negated {
		return !selected
	}
	return selected
}

func (a *sedAddress) selectsBeforeNegation(line string, number int, isLast bool) bool {
	if a.start.empty() {
		// No address at all: every line.
		return true
	}
	if !a.ranged {
		return a.start.matches(line, number, isLast)
	}
	if a.active && a.relative {
		a.active = number < a.until
		return true
	}
	if a.active {
		// The closing address is tested on a *later* line than the opening one,
		// so a one-line range like `2,2` still spans one line rather than
		// closing on itself immediately. A numeric end already passed means the
		// range closes here too, which is what keeps `4,2p` to one line.
		if a.end.matches(line, number, isLast) || (a.end.pattern == nil && !a.end.last && a.end.line <= number) {
			a.active = false
		}
		return true
	}
	if a.start.matches(line, number, isLast) {
		if a.relative {
			a.until = number + a.following
			a.active = a.following > 0
			return true
		}
		a.active = true
		// A numeric end at or before the start makes the range one line long,
		// so it is closed at once rather than left open to the end of the input.
		if a.end.pattern == nil && !a.end.last && a.end.line <= number {
			a.active = false
		}
		return true
	}
	return false
}

// atRangeEnd reports whether the line just selected is the last of a consecutive
// run this address will select.
//
// Only `c` needs it, and it needs it for a real reason: on a range, `c` prints its
// text once as the range closes rather than once per line, so
// `sed '1,2c\once'` answers a single `once`. A non-ranged address selects one
// line at a time, so every selection is its own end.
//
// Called after selects(), which is what leaves `active` describing the range's
// state *after* this line.
func (a *sedAddress) atRangeEnd() bool { return !a.ranged || !a.active }

// parseSedAddress reads the address in front of a command, returning what is
// left of the script.
func parseSedAddress(script string, extended bool) (sedAddress, string, error) {
	var address sedAddress
	rest := script
	start, rest, found, err := parseSedEndpoint(rest, extended)
	if err != nil {
		return address, "", err
	}
	if !found {
		return address, rest, nil
	}
	address.start = start
	if strings.HasPrefix(rest, ",+") && len(rest) > 2 && rest[2] >= '0' && rest[2] <= '9' {
		// `,+N`, as busybox reads it: digits after the plus, which a plus alone has not.
		digits := 2
		for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
			digits++
		}
		following, err := strconv.Atoi(rest[2:digits])
		if err != nil {
			return address, "", fmt.Errorf("invalid address: %s", rest[1:digits])
		}
		address.ranged, address.relative, address.following, rest = true, true, following, rest[digits:]
	} else if strings.HasPrefix(rest, ",") {
		end, remainder, endFound, err := parseSedEndpoint(rest[1:], extended)
		if err != nil {
			return address, "", err
		}
		if !endFound {
			// busybox's wording, measured: `sed -n '2,p'` answers
			// `sed: no address after comma`.
			return address, "", fmt.Errorf("no address after comma")
		}
		address.end, address.ranged, rest = end, true, remainder
		// A third comma is left alone deliberately, so it reaches the command
		// check and is reported as `unsupported command ,` -- which is what
		// busybox says for `1,2,3p`.
	}
	for strings.HasPrefix(rest, "!") {
		address.negated = !address.negated
		rest = rest[1:]
	}
	return address, rest, nil
}

// parseSedEndpoint reads one line number, `$`, or `/pattern/`.
func parseSedEndpoint(script string, extended bool) (sedEndpoint, string, bool, error) {
	var endpoint sedEndpoint
	switch {
	case script == "":
		return endpoint, script, false, nil
	case script[0] == '$':
		endpoint.last = true
		return endpoint, script[1:], true, nil
	case script[0] == '0':
		// Line numbering starts at 1, so there is no line zero to select. GNU
		// says `invalid usage of line address 0`; busybox instead lets `0`
		// parse as no address at all, so `sed -n '0p'` prints *every* line --
		// measured, and a quirk rather than a rule worth copying. The refusal
		// here names the address, where falling through to the command check
		// reported `unsupported command 0` and blamed the wrong thing.
		return endpoint, "", false, fmt.Errorf("invalid usage of line address 0")
	case script[0] >= '1' && script[0] <= '9':
		end := 0
		for end < len(script) && script[end] >= '0' && script[end] <= '9' {
			end++
		}
		value, err := strconv.Atoi(script[:end])
		if err != nil {
			return endpoint, "", false, fmt.Errorf("invalid address: %s", script[:end])
		}
		if end < len(script) && script[end] == '~' {
			// GNU's `first~step`, which busybox refuses by name and so does
			// this: answering it would need a different selector, and guessing
			// is worse than saying so.
			return endpoint, "", false, fmt.Errorf("unsupported command ~")
		}
		endpoint.line = value
		return endpoint, script[end:], true, nil
	case script[0] == '/':
		pattern, rest, err := readSedRegex(script[1:], '/')
		if err != nil {
			return endpoint, "", false, err
		}
		// An address pattern has no i flag to carry, so it never folds case.
		compiled, err := compileSedPattern(pattern, extended, false)
		if err != nil {
			return endpoint, "", false, err
		}
		endpoint.pattern = compiled
		return endpoint, rest, true, nil
	}
	return endpoint, script, false, nil
}

// readSedDelimited reads an s replacement or a y string up to its closing delimiter, and
// requires that the delimiter is actually there -- an unterminated `/a` is an error rather
// than text running to the end of the script.
//
// The end is found as busybox's index_of_next_unescaped_regexp_delim finds it: a backslash
// escapes the character after it, whatever it is, so `\\` is a pair and the `/` after it ends
// `s/$/\\/`. That was read as a backslash and an escaped delimiter, and refused as unmatched.
// What is kept is copy_parsing_escapes': `\n`, `\t` and `\r` are the characters, and the
// delimiter escaped is the delimiter, but for `&`, where `\&` stays escaped and so a literal
// ampersand in a replacement, busybox's and GNU's `s&8&\&&`. Any other escape is kept whole.
func readSedDelimited(script string, delimiter byte) (string, string, error) {
	var text strings.Builder
	for index := 0; index < len(script); index++ {
		char := script[index]
		switch {
		case char == delimiter:
			return text.String(), script[index+1:], nil
		case char == '\\' && index+1 < len(script):
			index++
			if control, ok := sedControlEscapes[script[index]]; ok {
				text.WriteString(control)
			} else if script[index] == delimiter && delimiter != '&' {
				text.WriteByte(delimiter)
			} else {
				text.WriteString(script[index-1 : index+1])
			}
		default:
			text.WriteByte(char)
		}
	}
	// busybox's wording, measured: `sed -n '/a'` answers `sed: unmatched '/'`.
	return "", "", fmt.Errorf("unmatched '%c'", delimiter)
}

// readSedRegex is readSedDelimited for a regular expression, scanned as busybox's
// index_of_next_unescaped_regexp_delim scans one: a bracket expression is opaque, so the
// delimiter inside one is a member of it, and `s/[/]/_/` replaces a slash where it ended the
// pattern at the bracket's slash. A `]` just after the `[` or `[^` is a member, not the end, and
// the delimiter escaped is the delimiter, in a bracket or out, as busybox's copy_parsing_escapes
// has it.
func readSedRegex(script string, delimiter byte) (string, string, error) {
	var text strings.Builder
	bracket := -1
	for index := 0; index < len(script); index++ {
		char := script[index]
		if char == '\\' && index+1 < len(script) && script[index+1] == delimiter {
			text.WriteByte(delimiter)
			index++
			continue
		}
		switch {
		case bracket >= 0:
			if char == ']' && index != bracket+1 && !(index == bracket+2 && script[index-1] == '^') {
				bracket = -1
			}
		case char == '\\' && index+1 < len(script):
			// An escape outside a bracket is two characters of the pattern, whatever the second.
			text.WriteByte(char)
			index++
			char = script[index]
		case char == '[':
			bracket = index
		case char == delimiter:
			return text.String(), script[index+1:], nil
		}
		text.WriteByte(char)
	}
	return "", "", fmt.Errorf("unmatched '%c'", delimiter)
}

// compileSedPattern honours -E: without it the pattern is BRE and goes through
// the translator sed_regex.go already has for `s///`.
//
// ignoreCase is the `s///i` flag. It is applied after any translation, so the
// case-folding wraps the finished expression rather than a BRE that has not been
// rewritten yet.
func compileSedPattern(pattern string, extended, ignoreCase bool) (*regexp.Regexp, error) {
	return compileRegex(pattern, extended, func(translated string) string {
		if ignoreCase {
			return "(?i)" + translated
		}
		return translated
	})
}
