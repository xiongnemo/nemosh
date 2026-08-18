package runtime

import "strings"

// Arithmetic sees the text after expansion, not before.
//
// `for ((i=0; i<${#a[@]}; i++))` -- the standard way to walk an array -- reported
// `arithmetic syntax error: unexpected "$"`, because the evaluator was handed the
// expression exactly as written and its lexer has no `$`. So did `$(( ${#a[@]} ))`
// and anything else naming a parameter with a dollar.
//
// The evaluator itself already reads a bare name (`$((i+1))` works), which is why
// this was easy to miss: the spelling without the dollar worked and the spelling
// with it did not, and both are common.
//
// bash expands the expression first, then evaluates the result. This does the same,
// for the reference forms this shell can expand without a context: `$name`,
// `${...}`, and the positional and special parameters. A command substitution inside
// arithmetic -- `$(( $(echo 2) * 3 ))` -- is still not handled, and is left as a `$`
// for the evaluator to refuse by name rather than silently becoming zero.

// expandArithmeticText replaces each parameter reference in an arithmetic expression
// with its value.
//
// Everything else is copied through untouched, including the operators and the
// blanks: the result goes to the arithmetic lexer, which has its own idea of what a
// token is, and rewriting anything but the references would be this function
// guessing at that.
func (r Runtime) expandArithmeticText(text string, savedStatus int) string {
	return r.expandEmbeddedParameters(text, savedStatus)
}

// expandEmbeddedParameters replaces each parameter reference in a piece of text with
// its value, leaving everything else alone.
//
// Shared by arithmetic and by the word of a parameter operator, because they need the
// same thing: `$(( $i + 1 ))` and `${x:-${y}}` are both text with references in it,
// and the operand path used to look the whole word up as a variable name -- so
// `${x:-${y}}` asked for a variable called `{y}` and found nothing.
func (r Runtime) expandEmbeddedParameters(text string, savedStatus int) string {
	if !strings.ContainsRune(text, '$') {
		return text
	}
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		if text[index] != '$' {
			out.WriteByte(text[index])
			continue
		}
		end := parameterEnd(text, index+1)
		if end <= index+1 {
			// A `$` that begins nothing this can read -- a command substitution,
			// most likely. Left as it was so the evaluator names it.
			out.WriteByte(text[index])
			continue
		}
		reference := text[index:end]
		values := r.expandParameterPart(wordPart{kind: wordPartParameter, text: reference}, savedStatus)
		// Joined with a blank, which is what an unquoted `${a[@]}` would produce
		// anyway. An expression using it as a number wants `${#a[@]}`, and one that
		// really does hold several numbers was never going to evaluate.
		out.WriteString(strings.Join(values, " "))
		index = end - 1
	}
	return out.String()
}
