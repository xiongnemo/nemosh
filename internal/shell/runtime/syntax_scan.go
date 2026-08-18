package runtime

import (
	"fmt"
	"strings"
)

type syntaxScanner struct {
	lines         []string
	logical       strings.Builder
	quotes        []byte
	substitutions int
	groupClosers  []byte
	syntaxErr     error
	escaped       bool
	continued     bool
}

// Line endings are normalized exactly once, on the way into parsing. ReplaceAll
// is not idempotent over a run of carriage returns — "one\r\r\n" becomes
// "one\r\n" and then "one\n" — so a second pass would eat a \r that is data.
func normalizeLineEndings(source string) string {
	return strings.ReplaceAll(source, "\r\n", "\n")
}

// The source must already have been through normalizeLineEndings.
func logicalLines(source string) ([]string, error) {
	physical := strings.Split(source, "\n")
	if len(physical) > 0 && physical[len(physical)-1] == "" {
		physical = physical[:len(physical)-1]
	}
	scanner := syntaxScanner{}
	for _, line := range physical {
		scanner.beginPhysicalLine()
		scanner.scanLine(line)
		scanner.finishPhysicalLine(line)
	}
	if err := scanner.incompleteError(); err != nil {
		return scanner.lines, err
	}
	scanner.flushLogicalLine()
	if scanner.syntaxErr != nil {
		return scanner.lines, scanner.syntaxErr
	}
	return scanner.lines, nil
}

func (scanner *syntaxScanner) beginPhysicalLine() {
	scanner.continued = false
}

func (scanner *syntaxScanner) scanLine(line string) {
	comment := false
	for index := 0; index < len(line); index++ {
		if scanner.syntaxErr != nil {
			return
		}
		char := line[index]
		if comment {
			continue
		}
		if scanner.escaped {
			scanner.logical.WriteByte(char)
			scanner.escaped = false
			continue
		}
		if scanner.quote() == '\'' {
			scanner.logical.WriteByte(char)
			if char == '\'' {
				scanner.popQuote()
			}
			continue
		}
		// Inside `$'...'` a backslash escapes, which is the whole difference from a
		// POSIX single-quoted string. Without this state the scanner read
		// `$'it\'s'` as a complete `'it\'` followed by an unterminated one and
		// called the script incomplete. See ansi_quote.go.
		if scanner.quote() == ansiQuoteMarker {
			scanner.logical.WriteByte(char)
			if char == '\\' && index+1 < len(line) {
				scanner.escaped = true
				continue
			}
			if char == '\'' {
				scanner.popQuote()
			}
			continue
		}
		if char == '$' && index+1 < len(line) && line[index+1] == '\'' && scanner.quote() == 0 {
			scanner.quotes = append(scanner.quotes, ansiQuoteMarker)
			scanner.logical.WriteByte(char)
			scanner.logical.WriteByte('\'')
			index++
			continue
		}
		if char == '\\' {
			if scanner.quote() != '\'' && index == len(line)-1 {
				scanner.continued = true
				continue
			}
			scanner.logical.WriteByte(char)
			scanner.escaped = true
			continue
		}
		if char == '\'' && scanner.quote() == 0 {
			scanner.quotes = append(scanner.quotes, char)
			scanner.logical.WriteByte(char)
			continue
		}
		if char == '"' {
			scanner.toggleDoubleQuote()
			scanner.logical.WriteByte(char)
			continue
		}
		// An arithmetic expansion is stepped over whole, before the command
		// substitution branch below can claim its first `(`. Otherwise the `))`
		// that closes it is counted as one substitution close and one group
		// close, and the group close is matched against whatever is really
		// open: `{ echo $((1+2)); }` fails with `unexpected ), expected }`.
		if char == '$' && index+2 < len(line) && line[index+1] == '(' && line[index+2] == '(' && scanner.quote() != '\'' {
			if end, ok := arithmeticExpansionEnd(line, index+3); ok {
				scanner.logical.WriteString(line[index : end+1])
				index = end
				continue
			}
		}
		if char == '$' && index+1 < len(line) && line[index+1] == '(' && scanner.quote() != '\'' {
			scanner.substitutions++
			scanner.quotes = append(scanner.quotes, 0)
			scanner.logical.WriteString("$(")
			index++
			continue
		}
		if char == ')' && scanner.substitutions > 0 && scanner.quote() == 0 {
			scanner.substitutions--
			scanner.popQuote()
			scanner.logical.WriteByte(char)
			continue
		}
		if scanner.quote() == 0 && scanner.substitutions == 0 && braceDelimiterAt(line, index, '{') {
			scanner.groupClosers = append(scanner.groupClosers, '}')
			scanner.logical.WriteByte(char)
			continue
		}
		if scanner.quote() == 0 && scanner.substitutions == 0 && char == '(' {
			// `a=(one two three)` is an array assignment, and the parentheses are
			// part of the word rather than a subshell. The scanner has to know
			// too, not only the lexer: it decides where a logical line ends, and
			// treating this `(` as a group opener made the `)` arrive at the
			// compound parser as a statement of its own -- `syntax error:
			// unexpected )`.
			if end, ok := arrayAssignmentSpan(line, index, scanner.logical.String()); ok {
				scanner.logical.WriteString(line[index : end+1])
				index = end
				continue
			}
			scanner.groupClosers = append(scanner.groupClosers, ')')
			scanner.logical.WriteByte(char)
			continue
		}
		if scanner.quote() == 0 && scanner.substitutions == 0 && (char == ')' || braceDelimiterAt(line, index, '}')) {
			if len(scanner.groupClosers) == 0 {
				scanner.logical.WriteByte(char)
				continue
			}
			expected := scanner.groupClosers[len(scanner.groupClosers)-1]
			if char != expected {
				scanner.syntaxErr = fmt.Errorf("syntax error: unexpected %c, expected %c", char, expected)
				return
			}
			scanner.groupClosers = scanner.groupClosers[:len(scanner.groupClosers)-1]
			scanner.logical.WriteByte(char)
			continue
		}
		if char == '#' && scanner.quote() == 0 && commentStarts(line, index) {
			comment = true
			continue
		}
		scanner.logical.WriteByte(char)
	}
}

func (scanner *syntaxScanner) finishPhysicalLine(line string) {
	if scanner.continued {
		return
	}
	if scanner.quote() != 0 || scanner.substitutions != 0 || len(scanner.groupClosers) != 0 {
		scanner.logical.WriteByte('\n')
		return
	}
	if hasTrailingSyntaxOperator(line) {
		scanner.logical.WriteByte(' ')
		scanner.continued = true
		return
	}
	scanner.flushLogicalLine()
}

// The cutset is deliberately not unicode.IsSpace: POSIX blanks are space and
// tab, a newline only ever joins physical lines here, and \r\n pairs are already
// gone by this point. Whatever \r survives is data, and trimming it would edit
// the user's word (docs/design/windows-execution-model.md).
const logicalLineCutset = " \t\n"

func (scanner *syntaxScanner) flushLogicalLine() {
	if scanner.syntaxErr != nil {
		return
	}
	segments, err := splitSequentialSegments(scanner.logical.String())
	scanner.logical.Reset()
	if err != nil {
		scanner.syntaxErr = err
		return
	}
	for _, segment := range segments {
		if normalized := strings.Trim(segment, logicalLineCutset); normalized != "" {
			scanner.lines = append(scanner.lines, splitLeadingReservedWord(normalized)...)
		}
	}
}

func (scanner *syntaxScanner) incompleteError() error {
	if scanner.syntaxErr != nil {
		return scanner.syntaxErr
	}
	if scanner.continued {
		return fmt.Errorf("%w: trailing line continuation", ErrIncompleteScript)
	}
	if scanner.quote() != 0 {
		return fmt.Errorf("%w: unterminated quote", ErrIncompleteScript)
	}
	if scanner.substitutions != 0 {
		return fmt.Errorf("%w: unterminated command substitution", ErrIncompleteScript)
	}
	if len(scanner.groupClosers) != 0 {
		return fmt.Errorf("%w: missing %c", ErrIncompleteScript, scanner.groupClosers[len(scanner.groupClosers)-1])
	}
	return nil
}

// ansiQuoteMarker stands for `$'...'` on the quote stack. Not a quote character,
// because it is not one: it is a state whose closing quote is `'` and in which a
// backslash escapes. Any non-zero value keeps the rest of the scanner treating the
// text as quoted, which is what it is.
const ansiQuoteMarker byte = 1

func (scanner *syntaxScanner) quote() byte {
	if len(scanner.quotes) == 0 {
		return 0
	}
	return scanner.quotes[len(scanner.quotes)-1]
}

func (scanner *syntaxScanner) popQuote() {
	scanner.quotes = scanner.quotes[:len(scanner.quotes)-1]
}

func (scanner *syntaxScanner) toggleDoubleQuote() {
	switch scanner.quote() {
	case '"':
		scanner.popQuote()
	case 0:
		scanner.quotes = append(scanner.quotes, '"')
	}
}

func commentStarts(line string, index int) bool {
	return index == 0 || line[index-1] == ' ' || line[index-1] == '\t'
}
