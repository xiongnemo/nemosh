package runtime

import (
	"fmt"
	"strings"
)

type syntaxScanner struct {
	lines []string
	// starts is, for each of lines, the physical line its text starts on (line_numbers.go).
	// logicalStart is the one the logical line in progress began on, and breaks the
	// offsets in it where each later physical line began.
	starts        []int
	lineBreaks    [][]int
	logicalStart  int
	breaks        []int
	logical       strings.Builder
	quotes        []byte
	substitutions []openSubstitution
	groupClosers  []byte
	// condition is a `[[` whose `]]` has not come; see followCondition.
	condition bool
	syntaxErr error
	escaped   bool
	continued bool
	joined    bool
}

// Line endings are normalized exactly once, on the way into parsing. ReplaceAll
// is not idempotent over a run of carriage returns — "one\r\r\n" becomes
// "one\r\n" and then "one\n" — so a second pass would eat a \r that is data.
func normalizeLineEndings(source string) string {
	return strings.ReplaceAll(source, "\r\n", "\n")
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
		if char == '$' && index+1 < len(line) && line[index+1] == '\'' && scanner.bare() {
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
		if char == '\'' && scanner.bare() {
			scanner.quotes = append(scanner.quotes, char)
			scanner.logical.WriteByte(char)
			continue
		}
		if char == '"' {
			scanner.toggleDoubleQuote()
			scanner.logical.WriteByte(char)
			continue
		}
		if char == '}' && scanner.quote() == braceParameterMarker {
			scanner.popQuote()
			scanner.logical.WriteByte(char)
			continue
		}
		// A `${...}` holds quotes of its own, and what they quote is no operator or comment:
		// `"${u:-"a ( b"}"`. Its `}` is no group's either, after a `;` as in `s=${s%;}`. Stepped
		// over whole when it closes on this line.
		if char == '$' && (scanner.quote() == '"' || scanner.bare()) && index+1 < len(line) && line[index+1] == '{' {
			if end, ok := bracedParameterEnd(line, index+1); ok {
				scanner.logical.WriteString(line[index:end])
				index = end - 1
				continue
			}
			if next, opened := scanner.openBraceParameter(line, index); opened {
				index = next
				continue
			}
		}
		// An arithmetic expansion that closes on this line is stepped over whole; see
		// stepArithmeticExpansion. One that goes on is an arithmetic span.
		if end, stepped := scanner.stepArithmeticExpansion(line, index); stepped {
			index = end
			continue
		}
		if char == '$' && index+1 < len(line) && line[index+1] == '(' && scanner.quote() != '\'' {
			scanner.quotes = append(scanner.quotes, 0)
			scanner.logical.WriteString("$(")
			arithmetic := index+2 < len(line) && line[index+2] == '('
			scanner.substitutions = append(scanner.substitutions, openSubstitution{body: scanner.logical.Len(), arithmetic: arithmetic})
			index++
			continue
		}
		if (char == '(' || char == ')') && len(scanner.substitutions) > 0 && scanner.quote() == 0 {
			scanner.substitutionParenthesis(char)
			scanner.logical.WriteByte(char)
			continue
		}
		if scanner.quote() == 0 && len(scanner.substitutions) == 0 && braceDelimiterAt(line, index, '{') {
			scanner.groupClosers = append(scanner.groupClosers, '}')
			scanner.logical.WriteByte(char)
			continue
		}
		if scanner.quote() == 0 && len(scanner.substitutions) == 0 && char == '(' {
			// `if(true)`: the reserved word ends at the `(`; see reservedWordBeforeParen.
			if reservedWordBeforeParen(line, index) {
				scanner.logical.WriteByte(' ')
			}
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
			// `((expr))` is an arithmetic command rather than two groups, and this
			// scanner has to know for the same reason it has to know about array
			// assignments: it decides where a logical line ends. See
			// arithmetic_command.go.
			if end := arithmeticCommandEnd(line, index); end > 0 {
				scanner.logical.WriteString(line[index : end+1])
				index = end
				continue
			}
			if end, ok := scanner.processSubstitution(line, index); ok {
				index = end
				continue
			}
			if wordGroupOpensAt(line, index) {
				end := skipBalancedParens(line, index)
				scanner.logical.WriteString(line[index:end])
				index = end - 1
				continue
			}
			if !scanner.casePattern() && scanner.openArithmeticCommand(line, index) {
				continue
			}
			if !scanner.casePattern() {
				scanner.groupClosers = append(scanner.groupClosers, ')')
			}
			scanner.logical.WriteByte(char)
			continue
		}
		if scanner.quote() == 0 && len(scanner.substitutions) == 0 && (char == ')' || braceDelimiterAt(line, index, '}')) {
			if len(scanner.groupClosers) == 0 || char == ')' && scanner.casePattern() {
				scanner.logical.WriteByte(char)
				continue
			}
			expected := scanner.groupClosers[len(scanner.groupClosers)-1]
			if char != expected {
				// A `)` while a `}` is open is a case pattern, not a mismatch:
				// `{ case a in a) x ;; esac; }` is ordinary and was reported as
				// `unexpected ), expected }`. The group's body is parsed by parseScript,
				// which knows about case arms, so passing it through is all that is
				// needed. The reverse -- a `}` while a `(` is open -- stays an error,
				// because a brace inside a subshell has no second meaning.
				if char == ')' && expected == '}' && insideCase(scanner.logical.String()) {
					scanner.logical.WriteByte(char)
					continue
				}
				scanner.syntaxErr = fmt.Errorf("syntax error: unexpected %c, expected %c", char, expected)
				return
			}
			scanner.groupClosers = scanner.groupClosers[:len(scanner.groupClosers)-1]
			scanner.logical.WriteByte(char)
			continue
		}
		if char == '#' && scanner.quote() == 0 && scanner.commentStartsAt(line, index) {
			comment = true
			continue
		}
		// `!(` where a command begins is `! (` to every scan after this one, a negated
		// subshell; see bangSubshellAt.
		if char == '!' && index+1 < len(line) && line[index+1] == '(' && scanner.quote() == 0 && bangSubshellAt(line, index) {
			scanner.logical.WriteString("! ")
			continue
		}
		scanner.followCondition(line, index)
		scanner.logical.WriteByte(char)
	}
}

func (scanner *syntaxScanner) finishPhysicalLine(line string) {
	if scanner.continued {
		return
	}
	if scanner.quote() != 0 || len(scanner.substitutions) != 0 || len(scanner.groupClosers) != 0 || scanner.condition {
		scanner.logical.WriteByte('\n')
		return
	}
	// Asked of the logical line so far, which knows the quotes and substitutions that began
	// on earlier lines and has no comments left in it. Asked of this line alone, `esac)" ||`
	// read its `"` as opening a quote, and a line after `&&` that was blank or a comment
	// ended the command there, both missing their second half.
	if hasTrailingSyntaxOperator(scanner.logical.String()) {
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
	case 0, braceParameterMarker:
		scanner.quotes = append(scanner.quotes, '"')
	}
}
