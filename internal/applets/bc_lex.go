package applets

import (
	"fmt"
	"strings"
)

// bc's tokens.
//
// **A number keeps its digits rather than its value.** `ibase` may change between the moment
// a program is read and the moment a number in it is evaluated, so converting at lex time
// would make `ibase=16; FF` a syntax error instead of 255. The text is carried through and
// converted by the evaluator with whatever base is then in force -- which is also why
// `ibase=16` itself works, since the `16` is converted while ibase is still ten.
//
// A newline is a token, because bc ends a statement with one; but it is **not** a token
// after an operator or an opening brace, where a statement is plainly unfinished.

type bcTokenKind uint8

const (
	bcTokenEOF bcTokenKind = iota
	bcTokenNumber
	bcTokenName
	bcTokenKeyword
	bcTokenString
	bcTokenOperator
	bcTokenNewline
)

type bcToken struct {
	kind bcTokenKind
	text string
	line int
}

var bcKeywords = map[string]bool{
	"define": true, "auto": true, "if": true, "else": true, "while": true,
	"for": true, "break": true, "continue": true, "return": true,
	"halt": true, "quit": true, "print": true,
	"length": true, "scale": true, "sqrt": true, "read": true,
}

// bcOperators are longest first, so `<=` is not read as `<` and then `=`, and `**` -- which
// some bcs accept for `^` -- is not read as two multiplications.
var bcOperators = []string{
	"&&", "||", "==", "!=", "<=", ">=", "+=", "-=", "*=", "/=", "%=", "^=", "++", "--",
	"+", "-", "*", "/", "%", "^", "=", "<", ">", "!", "(", ")", "{", "}", "[", "]", ",", ";",
}

type bcLexer struct {
	source string
	offset int
	line   int
}

func scanBcTokens(source string) ([]bcToken, error) {
	lexer := &bcLexer{source: source, line: 1}
	var tokens []bcToken
	for {
		token, err := lexer.next()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
		if token.kind == bcTokenEOF {
			return tokens, nil
		}
	}
}

func (l *bcLexer) next() (bcToken, error) {
	l.skipBlanks()
	if l.offset >= len(l.source) {
		return bcToken{kind: bcTokenEOF, line: l.line}, nil
	}
	character := l.source[l.offset]
	switch {
	case character == '\n':
		l.offset++
		line := l.line
		l.line++
		return bcToken{kind: bcTokenNewline, text: "\n", line: line}, nil
	case character == '"':
		return l.scanString()
	// A-F start a number: bc's digits above nine are upper case, and its names are
	// lower case, which is exactly so the two cannot be confused.
	case character >= '0' && character <= '9', character >= 'A' && character <= 'F':
		return l.scanNumber(), nil
	case character == '.' && l.digitFollows():
		return l.scanNumber(), nil
	case character == '.':
		// A lone `.` is `last`, the value the session printed most recently -- which is
		// why `5` and then `. + 1` answers 6. A `.` with a digit after it is the start of
		// a fraction, which is the case above.
		l.offset++
		return bcToken{kind: bcTokenName, text: "last", line: l.line}, nil
	case isBcNameStart(character):
		return l.scanName(), nil
	}
	return l.scanOperator()
}

func (l *bcLexer) digitFollows() bool {
	next := l.offset + 1
	return next < len(l.source) && l.source[next] >= '0' && l.source[next] <= '9'
}

func isBcNameStart(character byte) bool {
	return character >= 'a' && character <= 'z' || character == '_'
}

// skipBlanks passes over spaces, comments and a backslash-newline continuation.
//
// Two comment forms: `/* ... */`, which is POSIX, and `#` to the end of the line, which is
// not but which every bc accepts and every script uses.
func (l *bcLexer) skipBlanks() {
	for l.offset < len(l.source) {
		character := l.source[l.offset]
		switch {
		case character == ' ' || character == '\t' || character == '\r':
			l.offset++
		case character == '\\' && l.offset+1 < len(l.source) && l.source[l.offset+1] == '\n':
			l.offset += 2
			l.line++
		case character == '#':
			for l.offset < len(l.source) && l.source[l.offset] != '\n' {
				l.offset++
			}
		case character == '/' && l.offset+1 < len(l.source) && l.source[l.offset+1] == '*':
			l.skipBlockComment()
		default:
			return
		}
	}
}

func (l *bcLexer) skipBlockComment() {
	l.offset += 2
	for l.offset < len(l.source) {
		if l.source[l.offset] == '\n' {
			l.line++
		}
		if l.source[l.offset] == '*' && l.offset+1 < len(l.source) && l.source[l.offset+1] == '/' {
			l.offset += 2
			return
		}
		l.offset++
	}
}

// scanNumber takes the digits, including the letters a base above ten needs.
//
// Upper case only for the digit letters, which is what keeps `A` a digit and `a` a variable
// name -- bc's names are lower case for exactly this reason.
func (l *bcLexer) scanNumber() bcToken {
	start := l.offset
	for l.offset < len(l.source) {
		character := l.source[l.offset]
		isDigit := character >= '0' && character <= '9' || character >= 'A' && character <= 'F'
		if !isDigit && character != '.' {
			break
		}
		l.offset++
	}
	return bcToken{kind: bcTokenNumber, text: l.source[start:l.offset], line: l.line}
}

func (l *bcLexer) scanName() bcToken {
	start := l.offset
	for l.offset < len(l.source) {
		character := l.source[l.offset]
		if !isBcNameStart(character) && !(character >= '0' && character <= '9') {
			break
		}
		l.offset++
	}
	text := l.source[start:l.offset]
	if bcKeywords[text] {
		return bcToken{kind: bcTokenKeyword, text: text, line: l.line}
	}
	return bcToken{kind: bcTokenName, text: text, line: l.line}
}

// scanString reads `"..."`, which has **no escapes at all** in bc.
//
// A backslash is a backslash, so `print "a\nb"` prints the four characters. Both references
// agree, and it catches everybody who has used printf.
func (l *bcLexer) scanString() (bcToken, error) {
	line := l.line
	l.offset++
	start := l.offset
	for l.offset < len(l.source) {
		if l.source[l.offset] == '"' {
			text := l.source[start:l.offset]
			l.offset++
			return bcToken{kind: bcTokenString, text: text, line: line}, nil
		}
		if l.source[l.offset] == '\n' {
			l.line++
		}
		l.offset++
	}
	return bcToken{}, fmt.Errorf("line %d: unterminated string", line)
}

func (l *bcLexer) scanOperator() (bcToken, error) {
	rest := l.source[l.offset:]
	for _, operator := range bcOperators {
		if strings.HasPrefix(rest, operator) {
			l.offset += len(operator)
			return bcToken{kind: bcTokenOperator, text: operator, line: l.line}, nil
		}
	}
	return bcToken{}, fmt.Errorf("line %d: unexpected character '%c'", l.line, l.source[l.offset])
}
