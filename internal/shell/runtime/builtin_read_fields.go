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
			switch {
			case pendingEscape:
				pendingEscape = false
				// A backslash before the delimiter is a continuation: both
				// vanish and the line keeps going. Measured: `a\` then `b` reads
				// as `ab`.
				if char == options.delimiter {
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
// Measured against bash:
//
//	read a b      over `one two three`   a=one   b=two three
//	read a b      over `  a  b  c  `     a=a     b=b  c        -- inner run kept
//	IFS=: read a b over `a:b:c:d`        a=a     b=b:c:d
//	IFS=: read a b c over `a::b`         a=a     b=        c=b -- empty kept
//	IFS=: read a b c over `:a:`          a=      b=a       c=
//	IFS= read x   over ` a b `           x= a b            -- no splitting at all
//
// limit is how many fields to produce at most; 0 means as many as there are,
// which is what `-a` wants.
func splitReadFields(text string, escaped []bool, separators string, limit int) []string {
	// IFS set to the empty string turns splitting off, and with it the trimming:
	// the line arrives whole, leading and trailing blanks included.
	if separators == "" {
		return []string{text}
	}
	isSeparator := func(index int) bool {
		if index >= len(text) || (index < len(escaped) && escaped[index]) {
			return false
		}
		return strings.IndexByte(separators, text[index]) >= 0
	}
	isBlankSeparator := func(index int) bool {
		return isSeparator(index) && isFieldWhitespace(text[index])
	}
	start, end := 0, len(text)
	for start < end && isBlankSeparator(start) {
		start++
	}
	for end > start && isBlankSeparator(end-1) {
		end--
	}
	var fields []string
	position := start
	for {
		if limit > 0 && len(fields) == limit-1 {
			return append(fields, text[position:end])
		}
		fieldStart := position
		for position < end && !isSeparator(position) {
			position++
		}
		fields = append(fields, text[fieldStart:position])
		if position >= end {
			return fields
		}
		// A run of IFS whitespace is one delimiter. A non-whitespace separator is
		// a delimiter on its own, and may be trailed by whitespace that belongs
		// to the same one -- which is why `IFS=:` over `a: b` gives `a` and `b`.
		if isBlankSeparator(position) {
			for position < end && isBlankSeparator(position) {
				position++
			}
		} else {
			position++
			for position < end && isBlankSeparator(position) {
				position++
			}
		}
		if position >= end {
			// The line ended on a separator, so there is one more field and it
			// is empty. Only reachable for a non-whitespace separator, because a
			// trailing whitespace run was trimmed above.
			return append(fields, "")
		}
	}
}

func isFieldWhitespace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n'
}
