package runtime

import (
	"errors"
	"strings"
)

// splitSequentialSegments cuts one logical line at the `;` separators POSIX
// defines in 2.9.3, so `echo a; echo b` reaches the rest of the parser as the
// two lines it already knows how to run and `if true; then echo yes; fi`
// reaches it as four. That is the same trick normalizeGroupSeparators
// (parser_group.go:217) already plays inside a brace group body; this applies it
// at the top level, where the separator used to be rejected outright.
//
// `;;` is a case-arm terminator rather than a separator, so it survives whole as
// its own segment -- the exact form compoundSpans matches at parser.go:111.
//
// A `;` that belongs to a nested construct is left alone: quotes, `$(...)`,
// `${...}`, brace groups, and subshells are stepped over as units, and each of
// those bodies is split again when it is parsed in its own right.
func splitSequentialSegments(line string) ([]string, error) {
	var segments []string
	quote := byte(0)
	escaped := false
	// openers is the brackets still open, innermost last. A count is not enough: a `)`
	// closes a `(` and not a `{`, and a case pattern's `)` sits inside a brace group.
	var openers []byte
	// backgrounded is the segments a `&` ended rather than a `;`, which the check at the
	// end must not read as `a &; b`.
	backgrounded := map[int]bool{}
	start := 0
	for index := 0; index < len(line); index++ {
		char := line[index]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if char == '\'' && quote != '"' || char == '"' && quote != '\'' {
			if quote == char {
				quote = 0
			} else if quote == 0 {
				quote = char
			}
			continue
		}
		if quote != 0 {
			continue
		}
		if skip, ok := expansionEnd(line, index); ok {
			index = skip
			continue
		}
		// A bracket where a case pattern goes belongs to the pattern, and counting it here
		// is what took `( case a in a) x ;; esac )` apart: the pattern's `)` popped the
		// subshell, so the `;;` after it split at depth zero. The optional `(` before a
		// pattern is skipped for the same reason, which is what keeps `case a in (a) x ;;
		// esac` working -- it and its `)` cancel out rather than one of them counting.
		// This is the guard the comment below says could not be insideCase: a position
		// rather than a depth. See case_pattern_position.go.
		if (char == '(' || char == ')') && casePatternPosition(line[:index]) {
			continue
		}
		if char == '(' {
			if wordGroupOpensAt(line, index) {
				index = skipBalancedParens(line, index) - 1
				continue
			}
			openers = append(openers, '(')
			continue
		}
		if braceDelimiterAt(line, index, '{') {
			openers = append(openers, '{')
			continue
		}
		// A `)` closes only a `(`. Inside a brace group it is a case pattern --
		// `{ case a in a) x ;; esac; }` -- and popping the brace there dropped the depth
		// to zero, so the `;;` after it was split off as a separator and the group came
		// apart. Counting openers rather than tracking which is what let that happen.
		// No insideCase guard here, unlike the other two scans: knowing *which* opener
		// is on top is already enough. A pattern's `)` cannot pop a `{`, and the
		// optional `(` before a pattern pushes and pops itself. Adding the guard broke
		// `case a in (a) x ;; esac`, because then nothing popped that `(` and the `;;`
		// stopped splitting.
		if char == ')' && len(openers) > 0 && openers[len(openers)-1] == '(' {
			openers = openers[:len(openers)-1]
			continue
		}
		if braceDelimiterAt(line, index, '}') && len(openers) > 0 && openers[len(openers)-1] == '{' {
			openers = openers[:len(openers)-1]
			continue
		}
		// A background `&` ends a command as `;` does, and before a reserved word it has
		// to split as `;` does: `for i in 1 2; do sleep 1& done` left `done` inside the
		// segment, so the loop never closed and the script was "missing done". It stays
		// on its command, which is what makes that command the background one.
		if char == '&' && len(openers) == 0 && backgroundBeforeReservedWord(line, index) {
			backgrounded[len(segments)] = true
			segments = append(segments, line[start:index+1])
			start = index + 1
			continue
		}
		if char != ';' || len(openers) != 0 {
			continue
		}
		segments = append(segments, line[start:index])
		// `;;&` and `;&` are case-arm terminators, not `;;` or `;` followed by a
		// background `&`. Read the other way, `case a in a) echo one ;;& ...` put the
		// `&` at the start of the next segment and the whole line failed.
		if index+1 < len(line) && line[index+1] == ';' {
			terminator := ";;"
			index++
			if index+1 < len(line) && line[index+1] == '&' {
				terminator = ";;&"
				index++
			}
			segments = append(segments, terminator)
		} else if index+1 < len(line) && line[index+1] == '&' {
			segments = append(segments, ";&")
			index++
		}
		start = index + 1
	}
	segments = append(segments, line[start:])
	return segments, rejectSeparatorMisuse(segments, backgrounded)
}

// splitLeadingReservedWord peels `then`, `else`, or `do` off the front of a
// segment. compoundSpans matches those three as whole lines (parser.go:98)
// because until now they could only arrive that way; on a one-line compound the
// separator leaves the keyword sharing a segment with the first command of the
// body. They are reserved words in POSIX 2.4 rather than commands, so splitting
// them off is recovering the structure, not guessing at it.
func splitLeadingReservedWord(segment string) []string {
	for _, keyword := range [...]string{"then", "else", "do"} {
		rest, ok := compoundHeader(segment, keyword)
		if !ok {
			continue
		}
		if trimmed := strings.Trim(rest, logicalLineCutset); trimmed != "" {
			return []string{keyword, trimmed}
		}
	}
	return []string{segment}
}

// expansionEnd steps over a `$(...)` or `${...}` that starts at index and
// reports the last byte it occupies. An unterminated one is left to the scanner
// that reports it, so this only claims what it can measure.
func expansionEnd(line string, index int) (int, bool) {
	if line[index] != '$' || index+1 >= len(line) {
		return 0, false
	}
	switch line[index+1] {
	case '(':
		if index+2 < len(line) && line[index+2] == '(' {
			return arithmeticExpansionEnd(line, index+3)
		}
		return commandSubstitutionEnd(line, index+2)
	case '{':
		end := strings.IndexByte(line[index+2:], '}')
		if end < 0 {
			return 0, false
		}
		return index + 2 + end, true
	}
	return 0, false
}

// Two things can precede a separator and must not. Nothing at all is one:
// `; echo a` is a syntax error in dash and bash, though a trailing `echo a;` is
// an ordinary command and the blank that always precedes a `;;` is the arm
// ending rather than a command missing. A `&` is the other: it already
// terminates a list, so `echo a &; next` is rejected too. Only the last segment
// has no separator after it, so every earlier one is a candidate.
func rejectSeparatorMisuse(segments []string, backgrounded map[int]bool) error {
	for index, segment := range segments {
		if index == len(segments)-1 || backgrounded[index] {
			continue
		}
		trimmed := strings.Trim(segment, logicalLineCutset)
		if trimmed == "" && isCaseTerminator(segments[index+1]) {
			continue
		}
		// A case terminator is a segment of its own and ends in `&` for two of the
		// three spellings. It is not a command left dangling after a background
		// operator, which is what the check below is for.
		if isCaseTerminator(trimmed) {
			continue
		}
		// `true & ;;` ends the arm after a background command, which both references
		// take; only a `;` of its own after the `&` is the misuse.
		if trimmed == "" || endsWithBackgroundOperator(trimmed) && !isCaseTerminator(segments[index+1]) {
			return errors.New("syntax error: unexpected ;")
		}
	}
	return nil
}

// `&&` is a different token and is left to the and-or parser, which reports the
// command missing after it; an escaped `\&` is data.
func endsWithBackgroundOperator(segment string) bool {
	if !strings.HasSuffix(segment, "&") || strings.HasSuffix(segment, "&&") {
		return false
	}
	return !strings.HasSuffix(segment, "\\&")
}

// isCaseTerminator reports whether a segment is one of the three ways a case arm can
// end. `;;&` keeps testing the patterns after it and `;&` runs the next arm without
// testing; see parser_case.go.
func isCaseTerminator(segment string) bool {
	switch segment {
	case ";;", ";;&", ";&":
		return true
	}
	return false
}

// backgroundBeforeReservedWord reports a background `&` at index -- not `&&`, not part of
// a redirection -- with a reserved word after it: `done`, `fi`, `esac` and the rest.
func backgroundBeforeReservedWord(line string, index int) bool {
	// The second `&` of `&&` is not one: operatorText reads a character from its
	// neighbours, and a `&` before it is not among the ones it knows.
	if index > 0 && line[index-1] == '&' {
		return false
	}
	if operator, _ := operatorText(line, index); operator != "&" {
		return false
	}
	fields := strings.Fields(line[index+1:])
	return len(fields) > 0 && ReservedWord(strings.TrimRight(fields[0], ";"))
}
