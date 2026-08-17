package completionspec

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The value side of the reader. See toml.go for why this is written out rather
// than delegated to a library.

// readValue reads a string or a list of strings, and names what it found when it
// is neither.
func (r *tomlReader) readValue() (tomlValue, error) {
	// The line ending and a comment are checked before anything else, because
	// `key =` with nothing after it is a different mistake from `key = 30` and
	// deserves to be told apart: describing the absent value as an unsupported
	// kind sends its author looking for a type problem that is not there.
	if r.done() || r.source[r.pos] == '\n' || r.source[r.pos] == '\r' || r.source[r.pos] == '#' {
		return tomlValue{}, fmt.Errorf("line %d: a value is missing", r.line)
	}
	switch r.source[r.pos] {
	case '"', '\'':
		text, err := r.readString()
		return tomlValue{text: text}, err
	case '[':
		list, err := r.readList()
		return tomlValue{list: list, isList: true}, err
	}
	return tomlValue{}, fmt.Errorf("line %d: %s", r.line, describeUnsupported(r.rest()))
}

// describeUnsupported says which part of TOML this format leaves out, rather than
// only that the line is wrong. Someone writing a spec has every reason to expect
// a number to work; the answer is that it would have nowhere to go.
func describeUnsupported(rest string) string {
	switch {
	case strings.HasPrefix(rest, "{"):
		return "an inline table: this format has no inline tables, use a [table] header"
	case strings.HasPrefix(rest, "true"), strings.HasPrefix(rest, "false"):
		return "a boolean: every value in this format is a string or a list of strings"
	}
	word := rest
	if cut := strings.IndexAny(word, " \t\r\n#,]"); cut >= 0 {
		word = word[:cut]
	}
	return fmt.Sprintf("%q is not a string or a list of strings: this format has no numbers, booleans, dates or inline tables", word)
}

// readString reads a basic (`"`) or literal (`'`) string.
//
// Both are one line. TOML's triple-quoted forms are refused by name, because
// every value this format holds is a name, a letter set or a date, and none of
// them span lines.
func (r *tomlReader) readString() (string, error) {
	quote := r.source[r.pos]
	start := r.line
	if r.hasPrefix(strings.Repeat(string(quote), 3)) {
		return "", fmt.Errorf("line %d: a multi-line string; every value in this format fits on one line", start)
	}
	r.pos++
	var text strings.Builder
	for r.pos < len(r.source) {
		char := r.source[r.pos]
		switch {
		case char == quote:
			r.pos++
			return text.String(), nil
		case char == '\n':
			return "", fmt.Errorf("line %d: a string that is not closed before the end of the line", start)
		case char == '\\' && quote == '"':
			decoded, width, err := unescape(r.source[r.pos:])
			if err != nil {
				return "", fmt.Errorf("line %d: %w", r.line, err)
			}
			text.WriteString(decoded)
			r.pos += width
		default:
			text.WriteByte(char)
			r.pos++
		}
	}
	return "", fmt.Errorf("line %d: a string that is never closed", start)
}

// readList reads an array of strings.
//
// Line breaks and comments are allowed inside, because the real files need them:
// fastboot's long options are twenty-five names, and one line of them reads as a
// wall.
func (r *tomlReader) readList() ([]string, error) {
	start := r.line
	r.pos++
	list := []string{}
	for {
		r.skipSpaceAndComments()
		if r.done() {
			return nil, fmt.Errorf("line %d: a list that is never closed", start)
		}
		if r.source[r.pos] == ']' {
			r.pos++
			return list, nil
		}
		if r.source[r.pos] != '"' && r.source[r.pos] != '\'' {
			return nil, fmt.Errorf("line %d: a list in this format holds strings, and %s",
				r.line, describeUnsupported(r.rest()))
		}
		text, err := r.readString()
		if err != nil {
			return nil, err
		}
		list = append(list, text)
		r.skipSpaceAndComments()
		if r.done() {
			return nil, fmt.Errorf("line %d: a list that is never closed", start)
		}
		switch r.source[r.pos] {
		case ',':
			r.pos++
		case ']':
			r.pos++
			return list, nil
		default:
			return nil, fmt.Errorf("line %d: expected , or ] in a list, found %q", r.line, r.rest())
		}
	}
}

// unescape reads one backslash escape, returning its text and the bytes it took.
// The set is TOML's for a basic string.
func unescape(source []byte) (string, int, error) {
	if len(source) < 2 {
		return "", 0, errors.New("a backslash at the end of the file")
	}
	switch source[1] {
	case 'b':
		return "\b", 2, nil
	case 't':
		return "\t", 2, nil
	case 'n':
		return "\n", 2, nil
	case 'f':
		return "\f", 2, nil
	case 'r':
		return "\r", 2, nil
	case '"':
		return `"`, 2, nil
	case '\\':
		return `\`, 2, nil
	case 'u':
		return unescapeCodepoint(source, 4)
	case 'U':
		return unescapeCodepoint(source, 8)
	}
	return "", 0, fmt.Errorf(`\%c is not an escape TOML has`, source[1])
}

func unescapeCodepoint(source []byte, digits int) (string, int, error) {
	if len(source) < 2+digits {
		return "", 0, fmt.Errorf(`\%c needs %d hex digits`, source[1], digits)
	}
	text := string(source[2 : 2+digits])
	value, err := strconv.ParseUint(text, 16, 32)
	if err != nil {
		return "", 0, fmt.Errorf(`\%c%s is not %d hex digits`, source[1], text, digits)
	}
	// The surrogate halves parse as numbers and are not characters. Writing one
	// out would put a replacement character in a spec silently.
	if !utf8.ValidRune(rune(value)) {
		return "", 0, fmt.Errorf(`\%c%s is not a character`, source[1], text)
	}
	return string(rune(value)), 2 + digits, nil
}

func (r *tomlReader) hasPrefix(text string) bool {
	return strings.HasPrefix(string(r.source[r.pos:]), text)
}
