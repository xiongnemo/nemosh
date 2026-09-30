package runtime

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xiongnemo/nemosh/internal/shellquote"
)

// `${name@op}`, bash's parameter transformations: `@Q` quotes a value so the shell reads it
// back as itself, `@E` decodes its backslash escapes as `$'...'` would, `@U` `@u` `@L`
// change its case, `@A` writes the assignment that recreates it and `@a` its attributes.
// Each was `bad substitution`, and `${x@Q}` is how a script builds a command line that
// survives a value with a quote or a blank in it. bash is the reference; busybox has none.
//
// `@K` and `@k` write an array as its pairs, see parameter_kv_transform.go.
//
// `@P` reads the value as a prompt is read; see promptTransform.

// splitTransform recognises the form: a parameter reference, `@`, and one letter. Whole,
// rather than through splitParameterOperator, because `@` is a name as well as the
// operator -- `${@@Q}`, `${a[@]@Q}` -- and a search from the left finds the wrong one.
func splitTransform(body string) (string, byte, bool) {
	if len(body) < 3 || body[len(body)-2] != '@' {
		return "", 0, false
	}
	name, operator := body[:len(body)-2], body[len(body)-1]
	if !strings.ContainsRune("QEPAaUuLKk", rune(operator)) {
		return "", 0, false
	}
	if _, element := parseArrayReference(name); !element && !isBareParameterReference(name) {
		return "", 0, false
	}
	return name, operator, true
}

// transformParameter answers one value's transformation. An unset parameter transforms to
// nothing at all rather than to an empty pair of quotes, which is bash's answer.
func (r Runtime) transformParameter(ctx context.Context, name string, operator byte, value string, set bool, savedStatus int) (string, error) {
	if operator == 'a' {
		return r.attributeLetters(name), nil
	}
	if !set {
		return "", nil
	}
	switch operator {
	case 'P':
		return r.promptTransform(ctx, value, savedStatus), nil
	case 'Q', 'K', 'k':
		// K and k over anything but a whole array quote as Q does.
		return shellquote.Single(value), nil
	case 'E':
		return decodeAnsiText(value), nil
	case 'U':
		return strings.ToUpper(value), nil
	case 'L':
		return strings.ToLower(value), nil
	case 'u':
		first, width := utf8.DecodeRuneInString(value)
		return string(unicode.ToUpper(first)) + value[width:], nil
	default: // 'A'
		flags := strings.Map(func(letter rune) rune {
			if letter == 'a' || letter == 'A' {
				return -1
			}
			return letter
		}, r.attributeLetters(name))
		if flags == "" {
			return name + "=" + shellquote.Single(value), nil
		}
		return "declare -" + flags + " " + name + "=" + shellquote.Single(value), nil
	}
}

// attributeLetters is `${name@a}`: what the name was declared as, in bash's order.
func (r Runtime) attributeLetters(name string) string {
	if reference, ok := parseArrayReference(name); ok {
		name = reference.name
	}
	var letters strings.Builder
	switch {
	case r.arrays.isAssociative(name):
		letters.WriteByte('A')
	case r.hasIndexedArray(name):
		letters.WriteByte('a')
	}
	if r.isReadonly(name) {
		letters.WriteByte('r')
	}
	if r.isExported(name) {
		letters.WriteByte('x')
	}
	return letters.String()
}

func (r Runtime) hasIndexedArray(name string) bool {
	return r.arrays.has(name)
}

// transformList answers `${a[@]@Q}` and `${@@Q}`: each element transformed, except `@A`
// over an array, which is the one declaration that recreates the whole of it.
func (r Runtime) transformList(ctx context.Context, name string, operator byte, savedStatus int) ([]string, bool, error) {
	elements, isList := r.parameterList(ctx, name)
	if !isList {
		return nil, false, nil
	}
	if operator == 'A' {
		if reference, ok := parseArrayReference(name); ok {
			text, _ := r.declarationText(reference.name)
			return []string{text}, true, nil
		}
	}
	if operator == 'K' || operator == 'k' {
		if reference, ok := parseArrayReference(name); ok && (reference.subscript == "@" || reference.subscript == "*") {
			if words, isArray := r.keyValueWords(reference, operator); isArray {
				return words, true, nil
			}
		}
	}
	transformed := make([]string, 0, len(elements))
	for _, element := range elements {
		value, err := r.transformParameter(ctx, name, operator, element, true, savedStatus)
		if err != nil {
			return nil, true, err
		}
		transformed = append(transformed, value)
	}
	if reference, ok := parseArrayReference(name); (name == "*" || ok && reference.subscript == "*") && !r.starFields {
		// The `*` forms join into one word, transformed or not, unless unquoted; see
		// buildParameter.
		return []string{strings.Join(transformed, r.starSeparator())}, true, nil
	}
	return transformed, true, nil
}

// decodeAnsiText decodes backslash escapes the way `$'...'` does, over a whole value. A NUL
// one makes ends the text, as bash ends it; `@E` is bash's, which busybox has not got.
func decodeAnsiText(text string) string {
	var out strings.Builder
	for index := 0; index < len(text); {
		if text[index] != '\\' || index+1 >= len(text) {
			out.WriteByte(text[index])
			index++
			continue
		}
		decoded, width := decodeAnsiEscape(text[index:])
		if before, _, cut := strings.Cut(decoded, "\x00"); cut {
			out.WriteString(before)
			break
		}
		out.WriteString(decoded)
		index += width
	}
	return out.String()
}
