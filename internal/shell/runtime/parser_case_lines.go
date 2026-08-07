package runtime

import "strings"

// expandCaseArmLines rewrites the one-line forms of `case` into the line shape
// compoundSpans matches: `case WORD in` alone, then `PATTERN)` alone, then the
// arm body, then `;;`. Written across lines a case already arrives that way.
// Written on one line -- `case $1 in -h) usage;; esac` -- the header, the
// pattern, and the body share a segment, and splitSequentialSegments cannot
// separate them because the boundaries here are the reserved word `in` and the
// `)` after a pattern rather than a `;`.
//
// Only `case` and `esac` move the stack. Any other compound nested in an arm
// opens and closes unseen, which is enough: all the stack has to answer is
// which case an `esac` or `;;` belongs to, and whether that case is still
// waiting for the pattern that starts its next arm.
func expandCaseArmLines(lines []string) []string {
	var expanded []string
	var awaitingPattern []bool
	for _, line := range lines {
		for rest := line; rest != ""; {
			var emit string
			emit, rest = nextCaseLine(&awaitingPattern, rest)
			expanded = append(expanded, emit)
		}
	}
	return expanded
}

func nextCaseLine(stack *[]bool, line string) (string, string) {
	if header, ok := compoundHeader(line, "case"); ok {
		through, rest := splitAfterCaseIn(header)
		*stack = append(*stack, true)
		return "case " + through, rest
	}
	if caseCloserOrContinuation(line) {
		if len(*stack) > 0 {
			*stack = (*stack)[:len(*stack)-1]
		}
		return line, ""
	}
	if line == ";;" {
		if len(*stack) > 0 {
			(*stack)[len(*stack)-1] = true
		}
		return line, ""
	}
	if len(*stack) > 0 && (*stack)[len(*stack)-1] {
		if pattern, body, ok := splitCasePatternLine(line); ok {
			(*stack)[len(*stack)-1] = false
			return pattern, body
		}
	}
	return line, ""
}

// splitAfterCaseIn cuts a case header after its `in` reserved word. Everything
// through `in` is the header; anything after it is the first arm the one-line
// form crammed onto the same segment. A header with no `in` is returned whole,
// so parseTypedCase reports it rather than this pass guessing at it.
func splitAfterCaseIn(header string) (string, string) {
	unquoted := unquotedMask(header)
	start := -1
	for index := 0; index <= len(header); index++ {
		if index < len(header) && !(unquoted[index] && isShellBlank(header[index])) {
			if start < 0 {
				start = index
			}
			continue
		}
		if start >= 0 && header[start:index] == "in" {
			return header[:index], strings.TrimLeft(header[index:], " \t")
		}
		start = -1
	}
	return header, ""
}

// splitCasePatternLine cuts `PATTERN) body` at the `)` that ends the pattern.
func splitCasePatternLine(line string) (string, string, bool) {
	unquoted := unquotedMask(line)
	for index := 0; index < len(line); index++ {
		if unquoted[index] && line[index] == ')' {
			return line[:index+1], strings.TrimLeft(line[index+1:], " \t"), true
		}
	}
	return line, "", false
}

// splitCaseAlternatives cuts a case pattern at the top-level `|` POSIX 2.9.4.3
// uses to list alternatives, so `a|b)` offers two patterns for one arm.
func splitCaseAlternatives(pattern string) []string {
	unquoted := unquotedMask(pattern)
	var alternatives []string
	start := 0
	for index := 0; index < len(pattern); index++ {
		if !unquoted[index] || pattern[index] != '|' {
			continue
		}
		alternatives = append(alternatives, pattern[start:index])
		start = index + 1
	}
	return append(alternatives, pattern[start:])
}

// unquotedMask marks the byte positions of text that sit outside quotes and are
// not the escape or the escaped character of a backslash pair, so a scan
// looking for a reserved word or an operator can skip the ones that are data.
// The quote characters themselves stay unmarked; they delimit rather than
// participate.
func unquotedMask(text string) []bool {
	mask := make([]bool, len(text))
	quote := byte(0)
	escaped := false
	for index := 0; index < len(text); index++ {
		char := text[index]
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
		mask[index] = quote == 0
	}
	return mask
}
