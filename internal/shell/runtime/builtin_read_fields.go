package runtime

import (
	"context"
	"errors"
	"io"
	"strings"
)

// Collecting the line, and cutting it into fields.
//
// The two are one problem rather than two, because a backslash-escaped separator
// must survive splitting: measured against bash, `read p q` over `a\ b c` gives
// `a b` and `c`, not `a` and `b c`. So the collector records *which* bytes were
// escaped and the splitter honours that mask. Unescaping first and splitting
// afterwards cannot express it -- by then the space is an ordinary space.

// readLineResult is a line as read: the text with backslashes already removed,
// a mask marking the bytes that had been escaped, and whether the delimiter was
// actually reached.
type readLineResult struct {
	text string
	// escaped[i] reports that text[i] arrived behind a backslash, so it is data
	// and never a field separator.
	escaped []bool
	// delimited is false at end of input. bash returns 1 then, and still assigns
	// what it managed to read -- `printf a | read x` leaves x as `a` and fails.
	delimited bool
}

func collectReadLine(ctx context.Context, input io.Reader, options readOptions) (readLineResult, error) {
	var text []byte
	var escaped []bool
	buffer := []byte{0}
	pendingEscape := false
	for {
		if options.limit >= 0 && len(text) >= options.limit {
			return readLineResult{text: string(text), escaped: escaped, delimited: true}, nil
		}
		count, err := readWithContext(ctx, input, buffer)
		if count > 0 {
			char := buffer[0]
			// A NUL is dropped, as both references drop it, unless it is the delimiter:
			// `read -d ''` reads up to one.
			if char == 0 && options.delimiter != 0 {
				continue
			}
			switch {
			case pendingEscape:
				pendingEscape = false
				// A backslash before a newline is a continuation, whatever the delimiter:
				// both vanish and the line keeps going. Measured: `a\` then `b` reads as
				// `ab`, under `-d ,` too, where `\,` is a comma and ends nothing, as both
				// references read it. It was the delimiter that the backslash swallowed.
				if char == '\n' {
					continue
				}
				text, escaped = append(text, char), append(escaped, true)
			case !options.raw && char == '\\':
				pendingEscape = true
			case char == options.delimiter && !options.exactly:
				// -N reads a byte count and nothing else stops it, which is why
				// the delimiter is only honoured otherwise.
				return readLineResult{
					text: trimCarriageReturn(string(text), options), escaped: escaped, delimited: true,
				}, nil
			default:
				text, escaped = append(text, char), append(escaped, false)
			}
			continue
		}
		if errors.Is(err, io.EOF) {
			// A backslash with nothing after it is data, which is the only thing
			// left to do with it.
			if pendingEscape {
				text, escaped = append(text, '\\'), append(escaped, false)
			}
			return readLineResult{text: string(text), escaped: escaped}, nil
		}
		if err != nil {
			return readLineResult{}, err
		}
	}
}

// trimCarriageReturn drops the CR of a CRLF line ending. Windows text files are
// the common case here, and a variable holding an invisible CR is the kind of
// bug that shows up three commands later as a comparison that cannot be right.
// Only for a newline delimiter: someone who asked for `-d :` gets their bytes.
func trimCarriageReturn(text string, options readOptions) string {
	if options.delimiter != '\n' {
		return text
	}
	return strings.TrimSuffix(text, "\r")
}

// splitReadFields cuts a line for `read`, which is not quite ordinary field
// splitting: the *last* name takes everything left, separators and all.
//
// Measured against bash, and busybox-w32 answers the same:
//
//	read a b      over `one two three`   a=one   b=two three
//	read a b      over `  a  b  c  `     a=a     b=b  c        -- inner run kept
//	IFS=: read a b over `a:b:c:d`        a=a     b=b:c:d
//	IFS=: read a b over `a:b:`           a=a     b=b           -- one field left
//	IFS=: read a b c over `a::b`         a=a     b=        c=b -- empty kept
//	IFS=: read a b c over `:a:`          a=      b=a       c=
//	IFS=': ' read a b over `a : b :  `   a=a     b=b
//	IFS= read x   over ` a b `           x= a b            -- no splitting at all
//
// limit is how many fields to produce at most; 0 means as many as there are,
// which is what `-a` wants.
//
// The last name gets the rest of the line less the IFS whitespace at its end, unless what is
// left is a single field, which it gets without the delimiter after it: bash's read checks
// whether the fields are exactly as many as the names. The rest was taken whole, so `IFS=:
// read a b` over `a:b:` left b as `b:`.
func splitReadFields(text string, escaped []bool, separators string, limit int) []string {
	line := readSplitter{text: text, escaped: escaped, separators: separators}
	if limit == 0 {
		return line.fields()
	}
	var fields []string
	position := line.skipBlanks(0)
	for len(fields) < limit-1 && position < len(text) {
		var field string
		field, position = line.field(position)
		fields = append(fields, field)
	}
	if position >= len(text) {
		return fields
	}
	if field, next := line.field(position); next == len(text) {
		return append(fields, field)
	}
	end := len(text)
	for end > position && line.blank(end-1) {
		end--
	}
	return append(fields, text[position:end])
}

// readSplitter is a line read and the IFS it is split by. An escaped byte is data, never a
// separator; see readLineResult. IFS empty splits nothing, and the line arrives whole, its
// blanks and all.
type readSplitter struct {
	text       string
	escaped    []bool
	separators string
}

func (s readSplitter) separator(index int) bool {
	if index >= len(s.text) || (index < len(s.escaped) && s.escaped[index]) {
		return false
	}
	return strings.IndexByte(s.separators, s.text[index]) >= 0
}

func (s readSplitter) blank(index int) bool {
	return s.separator(index) && isFieldWhitespace(s.text[index])
}

func (s readSplitter) skipBlanks(position int) int {
	for position < len(s.text) && s.blank(position) {
		position++
	}
	return position
}

// field is the field at position and where the next one starts, past its delimiter: the
// separator that ends it and the IFS whitespace after that, and, when that separator was
// whitespace, a separator that is not and the whitespace after it too -- POSIX 2.6.5's "IFS
// white space ... adjacent" to one. Whitespace before a `:` was a delimiter of its own, so
// `IFS=': '` over `a : b` gave a, an empty field, and b.
func (s readSplitter) field(position int) (string, int) {
	start := position
	for position < len(s.text) && !s.separator(position) {
		position++
	}
	value := s.text[start:position]
	if position == len(s.text) {
		return value, position
	}
	blank := s.blank(position)
	position = s.skipBlanks(position + 1)
	if blank && s.separator(position) {
		position = s.skipBlanks(position + 1)
	}
	return value, position
}

// fields is every field of the line, for `read -a`: one that ends in a delimiter ends there,
// as a word's expansion does, and an empty line is none at all. Each made an empty field more.
func (s readSplitter) fields() []string {
	var fields []string
	position := s.skipBlanks(0)
	for position < len(s.text) {
		var value string
		value, position = s.field(position)
		fields = append(fields, value)
	}
	return fields
}

func isFieldWhitespace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n'
}
