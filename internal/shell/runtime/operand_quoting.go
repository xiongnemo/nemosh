package runtime

import (
	"context"
	"strings"
)

// The quotes inside a `${name OP word}` word.
//
// They were left in the word: `${x:-"a b"}` gave `"a b"` with its quotes, `${s#"a"}` looked
// for a prefix beginning with a quote character and removed nothing, and `${path%"/"}` --
// the everyday way to take a trailing slash off -- left it on. The rules are busybox-w32's
// and bash's, which agree on every case measured:
//
//   - a value (`:-`, `:+`, `:=`, `:?`) has its quotes removed. Inside a double-quoted
//     `${...}` a single quote is an ordinary character, and what it encloses expands;
//   - a pattern (`#`, `%`, `/`'s pattern, `^`, `,`) matches what is quoted literally, and
//     there single quotes are quotes even inside double quotes. An unquoted expansion in
//     it is still a pattern: `${s#$p}` with p='*' matches as `*` does;
//   - a replacement (`/`'s second half) has its quotes removed, single ones included.
//
// Backslashes follow the same split: a pattern keeps `\x` for the matcher, which reads it as
// a literal x, and a value drops the backslash -- inside double quotes only before one of
// $ ` " \ and a newline, as a double-quoted word does.

type operandRole uint8

const (
	operandValue operandRole = iota
	operandPattern
	operandReplacement
)

// expandOperand expands an operator's word in the role it plays.
func (r Runtime) expandOperand(ctx context.Context, word string, role operandRole, savedStatus int) string {
	if !strings.ContainsAny(word, "\"'\\") {
		return r.expandScalarParameterText(ctx, word, savedStatus)
	}
	var out strings.Builder
	for index := 0; index < len(word); {
		char := word[index]
		switch {
		case char == '\\' && index+1 < len(word):
			next := word[index+1]
			switch {
			case role == operandPattern:
				out.WriteString(word[index : index+2])
			case r.operandQuoted && strings.IndexByte("$`\"\\\n", next) < 0:
				out.WriteString(word[index : index+2])
			default:
				out.WriteByte(next)
			}
			index += 2
		case char == '\'' && !(r.operandQuoted && role == operandValue):
			end := strings.IndexByte(word[index+1:], '\'')
			if end < 0 {
				out.WriteString(word[index:])
				return out.String()
			}
			out.WriteString(literalIn(role, word[index+1:index+1+end]))
			index += end + 2
		case char == '"':
			end := doubleQuoteEnd(word, index+1)
			if end < 0 {
				out.WriteString(word[index:])
				return out.String()
			}
			out.WriteString(literalIn(role, r.expandDoubleQuotedText(ctx, word[index+1:end], savedStatus)))
			index = end + 1
		case char == '$':
			end := expansionEndAt(word, index)
			out.WriteString(r.expandEmbeddedParameters(ctx, word[index:end], savedStatus))
			index = end
		default:
			out.WriteByte(char)
			index++
		}
	}
	return out.String()
}

// expandReplaceSpec is `/`'s word: a pattern, then the replacement after the first `/`
// that is neither quoted nor escaped. Put back together with the pattern's own slashes
// escaped, which is how splitReplacementSpec reads a literal one.
func (r Runtime) expandReplaceSpec(ctx context.Context, word string, savedStatus int) string {
	cut := unquotedSlash(word)
	if cut < 0 {
		return r.expandOperand(ctx, word, operandPattern, savedStatus)
	}
	pattern := strings.ReplaceAll(r.expandOperand(ctx, word[:cut], operandPattern, savedStatus), "/", `\/`)
	return pattern + "/" + r.expandOperand(ctx, word[cut+1:], operandReplacement, savedStatus)
}

// expandDoubleQuotedText is what the inside of a double-quoted piece becomes: expansions
// made, and a backslash dropped where double quotes make it special.
func (r Runtime) expandDoubleQuotedText(ctx context.Context, text string, savedStatus int) string {
	var out strings.Builder
	for index := 0; index < len(text); {
		switch char := text[index]; {
		case char == '\\' && index+1 < len(text) && strings.IndexByte("$`\"\\\n", text[index+1]) >= 0:
			out.WriteByte(text[index+1])
			index += 2
		case char == '$':
			end := expansionEndAt(text, index)
			out.WriteString(r.expandEmbeddedParameters(ctx, text[index:end], savedStatus))
			index = end
		default:
			out.WriteByte(char)
			index++
		}
	}
	return out.String()
}

// literalIn is quoted text as the role needs it: escaped for a pattern, so it matches
// itself, and as it is otherwise.
func literalIn(role operandRole, text string) string {
	if role != operandPattern {
		return text
	}
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		if strings.IndexByte(`*?[]\`, text[index]) >= 0 {
			out.WriteByte('\\')
		}
		out.WriteByte(text[index])
	}
	return out.String()
}

// expansionEndAt is where the `$` expansion at index ends, or just past the `$` when it
// begins none.
func expansionEndAt(text string, index int) int {
	switch {
	case strings.HasPrefix(text[index:], "$(("):
		if end, ok := arithmeticExpansionEnd(text, index+3); ok {
			return end + 1
		}
	case strings.HasPrefix(text[index:], "$("):
		if end, ok := commandSubstitutionEnd(text, index+2); ok {
			return end + 1
		}
	}
	if end := parameterEnd(text, index+1); end > index+1 {
		return end
	}
	return index + 1
}

// doubleQuoteEnd is the closing quote of a double-quoted piece that opened before start,
// stepping over escapes and expansions, which may hold quotes of their own; -1 if none.
func doubleQuoteEnd(text string, start int) int {
	for index := start; index < len(text); index++ {
		switch text[index] {
		case '\\':
			index++
		case '"':
			return index
		case '$':
			index = expansionEndAt(text, index) - 1
		}
	}
	return -1
}

// unquotedSlash is the first `/` outside quotes, escapes and expansions, or -1.
func unquotedSlash(text string) int {
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '\\':
			index++
		case '\'':
			end := strings.IndexByte(text[index+1:], '\'')
			if end < 0 {
				return -1
			}
			index += end + 1
		case '"':
			end := doubleQuoteEnd(text, index+1)
			if end < 0 {
				return -1
			}
			index = end
		case '$':
			index = expansionEndAt(text, index) - 1
		case '/':
			return index
		}
	}
	return -1
}
