package applets

import (
	"fmt"
	"strconv"
	"strings"
)

// Scanning an awk program.
//
// Two things make this harder than it looks, and both were measured against gawk 5.4.1
// and busybox-w32 1.38.0 rather than recalled:
//
//   - **`/` is division or the start of a regular expression**, decided by what came
//     before it. See awkSlashIsDivision; the classic `a /b/ 2` settles it.
//   - **A newline ends a statement** except after a few tokens. See awkContinuesLine.
//
// Both are decided here so the parser never sees an ambiguity. A lexer that deferred
// either would have to hand the parser a token it could not name.

type awkLexer struct {
	source string
	offset int
	line   int
	// previous is the last token handed out, which is what the two context rules read.
	previous awkToken
}

func newAwkLexer(source string) *awkLexer {
	return &awkLexer{source: source, line: 1}
}

// scanAwkTokens reads a whole program.
//
// All at once rather than on demand: awk programs are small -- a one-liner or a file of a
// few hundred lines -- and having the whole slice makes the parser's lookahead a slice
// index instead of a buffer.
func scanAwkTokens(source string) ([]awkToken, error) {
	lexer := newAwkLexer(source)
	tokens := make([]awkToken, 0, 64)
	for {
		token, err := lexer.next()
		if err != nil {
			return nil, err
		}
		if token.kind == awkTokenEOF {
			return append(tokens, token), nil
		}
		tokens = append(tokens, token)
	}
}

func (l *awkLexer) next() (awkToken, error) {
	if err := l.skipBlanks(); err != nil {
		return awkToken{}, err
	}
	if l.offset >= len(l.source) {
		return l.emit(awkToken{kind: awkTokenEOF, line: l.line}), nil
	}
	character := l.source[l.offset]
	switch {
	case character == '\n':
		l.offset++
		token := awkToken{kind: awkTokenNewline, text: "\n", line: l.line}
		l.line++
		// A newline after a token that continues the line is not a terminator, so it
		// is swallowed here rather than handed on for the parser to ignore.
		if awkContinuesLine(l.previous) {
			return l.next()
		}
		return l.emit(token), nil
	case character == '"':
		return l.scanString()
	case character == '/' && !awkSlashIsDivision(l.previous):
		return l.scanRegex()
	case character >= '0' && character <= '9', character == '.' && l.digitFollows():
		return l.scanNumber()
	case character == '_' || isAwkNameByte(character):
		return l.scanName()
	}
	return l.scanOperator()
}

// skipBlanks consumes spaces, tabs, comments and backslash-newline continuations.
//
// A backslash-newline joins two lines anywhere, which both references confirm -- the
// first attempt to measure that reported both refusing it, and the fixture was wrong
// rather than the references.
func (l *awkLexer) skipBlanks() error {
	for l.offset < len(l.source) {
		switch l.source[l.offset] {
		case ' ', '\t', '\r':
			l.offset++
		case '#':
			for l.offset < len(l.source) && l.source[l.offset] != '\n' {
				l.offset++
			}
		case '\\':
			if l.offset+1 < len(l.source) && l.source[l.offset+1] == '\n' {
				l.offset += 2
				l.line++
				continue
			}
			if l.offset+2 < len(l.source) && l.source[l.offset+1] == '\r' && l.source[l.offset+2] == '\n' {
				l.offset += 3
				l.line++
				continue
			}
			return fmt.Errorf("line %d: backslash not followed by a newline", l.line)
		default:
			return nil
		}
	}
	return nil
}

func (l *awkLexer) digitFollows() bool {
	return l.offset+1 < len(l.source) && l.source[l.offset+1] >= '0' && l.source[l.offset+1] <= '9'
}

// emit records the token as the previous one and answers it.
func (l *awkLexer) emit(token awkToken) awkToken {
	l.previous = token
	return token
}

func (l *awkLexer) scanName() (awkToken, error) {
	start := l.offset
	for l.offset < len(l.source) && (isAwkNameByte(l.source[l.offset]) || isAwkDigit(l.source[l.offset])) {
		l.offset++
	}
	text := l.source[start:l.offset]
	switch {
	case awkKeywords[text]:
		return l.emit(awkToken{kind: awkTokenKeyword, text: text, line: l.line}), nil
	case awkBuiltins[text]:
		return l.emit(awkToken{kind: awkTokenBuiltin, text: text, line: l.line}), nil
	case l.offset < len(l.source) && l.source[l.offset] == '(':
		// No space before the parenthesis makes this a call. `f (x)` is a
		// concatenation of `f` and `x`; the difference really is the space.
		return l.emit(awkToken{kind: awkTokenFuncName, text: text, line: l.line}), nil
	}
	return l.emit(awkToken{kind: awkTokenName, text: text, line: l.line}), nil
}

// scanNumber reads a numeric literal, including the hex form.
//
// Hex **is** accepted here, and that is not a contradiction of awkScanNumber refusing it:
// both references print 16 for the source literal `0x10` while the string `"0x10"`
// converts to 0 under POSIX. gawk draws exactly this line -- hex as a source extension,
// decimal-only for conversion -- and measured, so does this.
func (l *awkLexer) scanNumber() (awkToken, error) {
	start := l.offset
	if strings.HasPrefix(strings.ToLower(l.source[l.offset:]), "0x") {
		l.offset += 2
		for l.offset < len(l.source) && isAwkHexDigit(l.source[l.offset]) {
			l.offset++
		}
		value, err := strconv.ParseUint(l.source[start+2:l.offset], 16, 64)
		if err != nil {
			return awkToken{}, fmt.Errorf("line %d: bad hexadecimal number %q", l.line, l.source[start:l.offset])
		}
		return l.emit(awkToken{kind: awkTokenNumber, text: l.source[start:l.offset], number: float64(value), line: l.line}), nil
	}
	end := awkNumberEnd(l.source[l.offset:])
	if end == 0 {
		return awkToken{}, fmt.Errorf("line %d: bad number", l.line)
	}
	l.offset += end
	value, err := strconv.ParseFloat(l.source[start:l.offset], 64)
	if err != nil {
		return awkToken{}, fmt.Errorf("line %d: bad number %q", l.line, l.source[start:l.offset])
	}
	return l.emit(awkToken{kind: awkTokenNumber, text: l.source[start:l.offset], number: value, line: l.line}), nil
}

// scanString reads a quoted string, decoding its escapes.
//
// Decoded here rather than at evaluation, so the rest of the program never carries a
// backslash it has to remember to interpret.
func (l *awkLexer) scanString() (awkToken, error) {
	line := l.line
	l.offset++
	var out strings.Builder
	for l.offset < len(l.source) {
		switch character := l.source[l.offset]; character {
		case '"':
			l.offset++
			return l.emit(awkToken{kind: awkTokenString, text: out.String(), line: line}), nil
		case '\n':
			return awkToken{}, fmt.Errorf("line %d: newline in string", line)
		case '\\':
			l.offset++
			decoded, width, err := awkDecodeEscape(l.source[l.offset:], line)
			if err != nil {
				return awkToken{}, err
			}
			out.WriteString(decoded)
			l.offset += width
		default:
			out.WriteByte(character)
			l.offset++
		}
	}
	return awkToken{}, fmt.Errorf("line %d: unterminated string", line)
}

// scanRegex reads `/re/`.
//
// `\/` is an escaped slash and stays a literal slash in the pattern; every other
// backslash is left **as written**, because the pattern is compiled by the regexp engine
// later and decoding `\t` here would hide `\.` from it.
func (l *awkLexer) scanRegex() (awkToken, error) {
	line := l.line
	l.offset++
	var out strings.Builder
	for l.offset < len(l.source) {
		switch character := l.source[l.offset]; character {
		case '/':
			l.offset++
			return l.emit(awkToken{kind: awkTokenRegex, text: out.String(), line: line}), nil
		case '\n':
			return awkToken{}, fmt.Errorf("line %d: newline in regular expression", line)
		case '\\':
			if l.offset+1 < len(l.source) && l.source[l.offset+1] == '/' {
				out.WriteByte('/')
				l.offset += 2
				continue
			}
			out.WriteByte('\\')
			l.offset++
		default:
			out.WriteByte(character)
			l.offset++
		}
	}
	return awkToken{}, fmt.Errorf("line %d: unterminated regular expression", line)
}

func (l *awkLexer) scanOperator() (awkToken, error) {
	rest := l.source[l.offset:]
	for _, operator := range awkOperators {
		if strings.HasPrefix(rest, operator) {
			l.offset += len(operator)
			return l.emit(awkToken{kind: awkTokenOperator, text: operator, line: l.line}), nil
		}
	}
	return awkToken{}, fmt.Errorf("line %d: unexpected character %q", l.line, rest[:1])
}

func isAwkNameByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_'
}

func isAwkDigit(b byte) bool { return b >= '0' && b <= '9' }

func isAwkHexDigit(b byte) bool {
	return isAwkDigit(b) || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}
