package runtime

import (
	"context"
	"strings"
)

// The word of `${name:-word}` and `${name:+word}` is expanded as a word of its own would be,
// straight into the fields being built: what it quotes stays whole, what it leaves unquoted
// is split, and `"$@"` in it is a field per parameter.
//
// It was expanded to one string and that string treated as the parameter's value, so its
// quoting was lost on the way out. Unquoted, `${u:-"a b"}` -- the everyday way to give a
// default with a blank in it -- was split into `a` and `b`, as were `${u:-'a b'}`,
// `${u:-a\ b}` and `${u:-"$y"}`. And "$@" inside was joined: `"${u:-$@}"` and
// `${u:-"$@"}` were one word where both references give one per parameter. busybox-w32 and
// bash agree on all of it.
//
// The same holds for the value when it is the parameter's own: `"${@:-x}"` is a field per
// parameter, and bash's `"${a[@]:-x}"` one per element. They were joined into one.
//
// The assigning and the erroring forms, `=` and `?`, still take a string: what `=` gives is
// the variable's new value, split like any other when it is unquoted, which both references
// agree on too.

// buildDefault adds a `-` or `+` expansion to the word being built, and reports whether the
// part was one.
func (r Runtime) buildDefault(ctx context.Context, build *fieldBuilder, part wordPart, savedStatus int) bool {
	name, operator, word, ok := defaultForm(part.text)
	if !ok {
		return false
	}
	quoted := part.quote == quoteDouble
	missing, list := r.defaultMissing(ctx, name, operator, quoted, savedStatus)
	switch {
	case missing == (operator == "-" || operator == ":-"):
		r.buildOperand(ctx, build, word, quoted, savedStatus)
	case missing:
		// `+` of something missing is nothing: one empty word quoted, none at all unquoted.
		if quoted {
			build.text("", false)
		}
	case list:
		r.buildParameter(ctx, build, wordPart{kind: wordPartParameter, text: listReference(name), quote: part.quote}, savedStatus)
	default:
		value, _ := r.operandParameter(ctx, name, savedStatus)
		build.expansion(value, part.quote)
	}
	return true
}

// defaultForm splits `${name-word}`, `${name:-word}`, `${name+word}` or `${name:+word}`.
func defaultForm(text string) (string, string, string, bool) {
	body, ok := strings.CutPrefix(text, "${")
	if !ok {
		return "", "", "", false
	}
	body, ok = strings.CutSuffix(body, "}")
	if !ok || body == "" || body[0] == '#' || body[0] == '!' || isBareParameterReference(body) {
		return "", "", "", false
	}
	if _, _, isTransform := splitTransform(body); isTransform {
		return "", "", "", false
	}
	name, operator, word, ok := splitParameterOperator(body)
	switch operator {
	case "-", ":-", "+", ":+":
		return name, operator, word, ok
	}
	return "", "", "", false
}

// defaultMissing reports whether the parameter counts as missing for the operator, and
// whether it is a list. The colon forms count an empty value as missing: for a list, one
// whose elements join to nothing, joined as the expansion would join them -- a quoted `*`
// form with IFS's first character, so `"${*:-x}"` of two empty parameters under an empty
// IFS is x, and anything else as `$@` is, where two empty words are something. The
// positional parameters are always set, as busybox has it; bash calls them unset when there
// are none.
//
// One answer here is bash's alone: under an empty IFS, unquoted `${*:+y}` of two empty
// parameters is y. busybox-w32 gives nothing there while taking the same parameters as set
// for `${*:-x}`, which cannot both be so.
func (r Runtime) defaultMissing(ctx context.Context, name, operator string, quoted bool, savedStatus int) (bool, bool) {
	colon := strings.HasPrefix(operator, ":")
	if elements, isList := r.parameterList(ctx, name); isList {
		switch {
		case colon && quoted && (name == "*" || strings.HasSuffix(name, "[*]")):
			return strings.Join(elements, r.starSeparator()) == "", true
		case colon:
			return strings.Join(elements, " ") == "", true
		case name == "@" || name == "*":
			return false, true
		}
		return len(elements) == 0, true
	}
	value, set := r.operandParameter(ctx, name, savedStatus)
	return !set || colon && value == "", false
}

// listReference is the plain reference to a list name: `$@`, `$*`, `${a[@]}`.
func listReference(name string) string {
	if name == "@" || name == "*" {
		return "$" + name
	}
	return "${" + name + "}"
}

// buildOperand adds an operator's word. quoted is whether the expansion it belongs to is in
// double quotes, which makes the whole word quoted: a double quote in it is only removed, and
// a single quote is an ordinary character, as it is in a double-quoted word.
func (r Runtime) buildOperand(ctx context.Context, build *fieldBuilder, word string, quoted bool, savedStatus int) {
	if quoted {
		// An expansion in double quotes is a word even when it comes to nothing.
		build.text("", false)
	}
	for index := 0; index < len(word); {
		char := word[index]
		switch {
		case char == '\\' && index+1 < len(word):
			switch next := word[index+1]; {
			case !quoted:
				build.text(word[index+1:index+2], false)
			case strings.IndexByte("$`\"\\\n", next) >= 0:
				build.text(word[index+1:index+2], false)
			default:
				build.text(word[index:index+2], false)
			}
			index += 2
		case char == '\'' && !quoted:
			end := strings.IndexByte(word[index+1:], '\'')
			if end < 0 {
				build.text(word[index:], false)
				return
			}
			build.text(word[index+1:index+1+end], false)
			index += end + 2
		case char == '"':
			end := doubleQuoteEnd(word, index+1)
			if end < 0 {
				build.text(word[index:], false)
				return
			}
			// `""` is an empty word; `"$@"` with no parameters is none, as everywhere.
			if end == index+1 {
				build.text("", false)
			}
			r.buildQuotedText(ctx, build, word[index+1:end], savedStatus)
			index = end + 1
		case char == '$':
			index = r.buildEmbedded(ctx, build, word, index, quoted, savedStatus)
		case quoted:
			build.text(word[index:index+1], false)
			index++
		default:
			build.split(word[index : index+1])
			index++
		}
	}
}

// buildQuotedText adds the inside of a double-quoted piece of an operator's word.
func (r Runtime) buildQuotedText(ctx context.Context, build *fieldBuilder, text string, savedStatus int) {
	for index := 0; index < len(text); {
		switch char := text[index]; {
		case char == '\\' && index+1 < len(text) && strings.IndexByte("$`\"\\\n", text[index+1]) >= 0:
			build.text(text[index+1:index+2], false)
			index += 2
		case char == '$':
			index = r.buildEmbedded(ctx, build, text, index, true, savedStatus)
		default:
			build.text(text[index:index+1], false)
			index++
		}
	}
}

// buildEmbedded adds the expansion that starts at the `$` at index, and answers where it
// ends. A parameter goes through buildParameter, so a list in it is fields and a default in
// it is this again.
func (r Runtime) buildEmbedded(ctx context.Context, build *fieldBuilder, text string, index int, quoted bool, savedStatus int) int {
	end := expansionEndAt(text, index)
	quote := quoteUnquoted
	if quoted {
		quote = quoteDouble
	}
	switch reference := text[index:end]; {
	case end == index+1:
		build.text("$", false)
	case strings.HasPrefix(reference, "$("):
		build.expansion(r.expandEmbeddedParameters(ctx, reference, savedStatus), quote)
	default:
		r.buildParameter(ctx, build, wordPart{kind: wordPartParameter, text: reference, quote: quote}, savedStatus)
	}
	return end
}
