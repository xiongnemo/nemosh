package runtime

import "strings"

// globEscaped is every character a quoted one is escaped from in a pattern; see escapeGlob.
const globEscaped = `*?[]\-()|!^`

// Field splitting (POSIX 2.6.5) across a whole word.
//
// An IFS character that an unquoted expansion produced delimits fields wherever it sits,
// including at either end of the expansion, where it divides the expansion from the text
// beside it. Each expansion was split on its own and its first and last pieces glued to
// their neighbours, so a separator at the edge was lost: with x='a ' and y=b, `$x$y` was the
// one field `ab`, `b$x` with x=' a' was `ba`, and `"<"$x">"` with x=' a b ' was `<a` and `b>`.
// Both references answer a field on each side of the separator.
//
// Three more rules went with it, and busybox-w32 and bash agree on every one:
//
//   - IFS whitespace beside a non-whitespace separator is part of that one delimiter, so
//     with IFS=' :' the value `a : b` is two fields, not three;
//   - a quoted empty string is a field of its own, even straight after a delimiter, so
//     `$x""` with x='a ' is `a` and an empty field;
//   - an unquoted expansion that comes to nothing is no field, IFS empty or not.
//
// fieldBuilder is that algorithm, fed a part at a time.
type fieldBuilder struct {
	// separators is IFS, and assignment is whether this word is the value of one, which
	// nothing splits.
	separators string
	assignment bool
	fields     []string
	// patterns is each field as a pathname pattern, its quoted characters escaped, or the
	// empty string for a field that is not one. See text.
	patterns []string
	current  strings.Builder
	pattern  strings.Builder
	// open is whether the field being built has begun: it holds a character, or a quoted
	// empty string, which begins one.
	open bool
	// glob is whether an unquoted pattern character is in the field being built.
	glob bool
	// delimiter is what ended the last field, while nothing has followed it: whitespace
	// there absorbs a non-whitespace separator after it, where another one would delimit
	// an empty field.
	delimiter delimiterKind
}

type delimiterKind uint8

const (
	noDelimiter delimiterKind = iota
	blankDelimiter
	hardDelimiter
)

func (r Runtime) newFieldBuilder() *fieldBuilder {
	return &fieldBuilder{separators: r.fieldSeparators(), assignment: r.noFieldSplit}
}

// text adds characters that stand as they are: a literal, a quoted piece, an expansion that
// is not split. Even an empty one begins a field, which is what makes `""` a word. glob is
// whether the characters are unquoted, and so may be a pattern.
//
// What is quoted goes into the field's pattern escaped, so a quoted pattern character is
// matched as itself when an unquoted one beside it makes the field a pattern: `\[???\]`
// matches a file named [abc], and `*.[C\-D]` a file ending in -, in both references. The
// pattern was the field's text, where the quoting had already gone.
func (b *fieldBuilder) text(value string, glob bool) {
	b.current.WriteString(value)
	if glob {
		b.pattern.WriteString(value)
	} else {
		b.pattern.WriteString(escapeGlob(value))
	}
	b.open = true
	b.glob = b.glob || glob && containsGlobMeta(value)
	b.delimiter = noDelimiter
}

// split adds what an unquoted expansion produced, which its IFS characters divide.
func (b *fieldBuilder) split(value string) {
	if b.assignment || b.separators == "" {
		if value != "" {
			b.text(value, true)
		}
		return
	}
	for index := 0; index < len(value); index++ {
		// A run of characters that are not separators goes in whole, since a command
		// substitution's output can be large.
		end := index
		for end < len(value) && strings.IndexByte(b.separators, value[end]) < 0 {
			end++
		}
		if end > index {
			next := byte(0)
			if end < len(value) {
				next = value[end]
			}
			if text := beforeNewline(value[index:end], next); text != "" {
				b.text(text, true)
			}
			index = end - 1
			continue
		}
		switch char := value[index]; {
		case char == ' ' || char == '\t' || char == '\n':
			if b.open {
				b.close()
				b.delimiter = blankDelimiter
			}
		default:
			if b.open || b.delimiter != blankDelimiter {
				b.close()
			}
			b.delimiter = hardDelimiter
		}
	}
}

// expansion adds an expansion's value, split if it was unquoted.
func (b *fieldBuilder) expansion(value string, quote quoteContext) {
	if quote == quoteUnquoted {
		b.split(value)
		return
	}
	b.text(value, false)
}

// quotedList adds `"$@"` or `"${a[@]}"`: a field per element, the first joining what came
// before it and the last what comes after. No elements is no field at all.
func (b *fieldBuilder) quotedList(values []string) {
	for index, value := range values {
		if index > 0 {
			b.close()
		}
		b.text(value, false)
	}
}

// unquotedList adds `$@` or `${a[@]}` unquoted: each element split in turn, with a field
// boundary between elements even when IFS is empty, and an empty element vanishing. In an
// assignment, which does not split, they are one value, joined by joiner.
func (b *fieldBuilder) unquotedList(values []string, joiner string) {
	if b.assignment {
		b.text(strings.Join(values, joiner), false)
		return
	}
	for index, value := range values {
		if index > 0 && b.open {
			b.close()
		}
		b.split(value)
	}
}

// close ends the field being built, empty or not.
func (b *fieldBuilder) close() {
	b.fields = append(b.fields, b.current.String())
	pattern := ""
	// An extended group is looked for in the whole field too, since it can arrive in pieces:
	// a default's word is added a character at a time, so `${u:-@(a|b)}` never globbed.
	if b.glob || hasExtendedPattern(b.pattern.String()) {
		pattern = b.pattern.String()
	}
	b.patterns = append(b.patterns, pattern)
	b.current.Reset()
	b.pattern.Reset()
	b.open, b.glob = false, false
}

// finish is the word's fields, and each one's pattern, empty for a field that is not one. A
// delimiter at the end of the word leaves no empty field after it.
func (b *fieldBuilder) finish() ([]string, []string) {
	if b.open {
		b.close()
	}
	return b.fields, b.patterns
}

// escapeGlob is text as a pattern that matches it and nothing else: each pattern character
// escaped, the hyphen too, which inside a bracket expression would make a range, and the
// parentheses and bar, so a quoted `@(a|b)` is those six characters and not a group. And ! and
// ^, which after a bracket's `[` would negate it: `case '!' in [\!])` matches, in busybox and
// bash, where the set was taken for "not ]" and never closed.
func escapeGlob(text string) string {
	if !strings.ContainsAny(text, globEscaped) {
		return text
	}
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		if strings.IndexByte(globEscaped, text[index]) >= 0 {
			out.WriteByte('\\')
		}
		out.WriteByte(text[index])
	}
	return out.String()
}
