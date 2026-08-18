package runtime

import (
	"context"
	"strings"
)

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
func (r Runtime) expandArithmeticText(ctx context.Context, text string, savedStatus int) string {
	return r.expandEmbeddedParameters(ctx, text, savedStatus)
}

// expandEmbeddedParameters replaces each parameter reference in a piece of text with
// its value, leaving everything else alone.
//
// Shared by arithmetic and by the word of a parameter operator, because they need the
// same thing: `$(( $i + 1 ))` and `${x:-${y}}` are both text with references in it,
// and the operand path used to look the whole word up as a variable name -- so
// `${x:-${y}}` asked for a variable called `{y}` and found nothing.
func (r Runtime) expandEmbeddedParameters(ctx context.Context, text string, savedStatus int) string {
	if !strings.ContainsRune(text, '$') {
		return text
	}
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		if text[index] != '$' {
			out.WriteByte(text[index])
			continue
		}
		// `$(...)` -- a command substitution. `$(( $(cmd) * 3 ))` reported
		// `unexpected "$"` and `${x:-$(cmd)}` printed the text at the reader, because
		// this walk had nothing for it and left the dollar standing.
		if index+1 < len(text) && text[index+1] == '(' {
			if end, ok := commandSubstitutionEnd(text, index+2); ok {
				out.WriteString(r.runEmbeddedSubstitution(ctx, text[index+2:end], savedStatus))
				index = end
				continue
			}
		}
		end := parameterEnd(text, index+1)
		if end <= index+1 {
			// A `$` that begins nothing this can read. Left as it was, so whatever
			// consumes the result names it rather than this silently dropping it.
			out.WriteByte(text[index])
			continue
		}
		reference := text[index:end]
		values := r.expandParameterPart(ctx, wordPart{kind: wordPartParameter, text: reference}, savedStatus)
		// Joined with a blank, which is what an unquoted `${a[@]}` would produce
		// anyway. An expression using it as a number wants `${#a[@]}`, and one that
		// really does hold several numbers was never going to evaluate.
		out.WriteString(strings.Join(values, " "))
		index = end - 1
	}
	return out.String()
}

// runEmbeddedSubstitution runs a `$(...)` found inside a piece of text and answers
// with its output, trailing newlines removed.
//
// It parses and runs through the same path a command substitution in an ordinary word
// takes, so the two cannot disagree about what `$(echo a; echo b)` produces. A script
// that will not parse yields the empty string rather than a diagnostic, because this
// runs while a word is being built and there is nowhere here to report to -- the same
// arrangement expandBracedParameter's callers already live with.
func (r Runtime) runEmbeddedSubstitution(ctx context.Context, script string, savedStatus int) string {
	parsed, err := ParseScript(script)
	if err != nil {
		return ""
	}
	return r.commandSubstitutionScript(ctx, parsed, savedStatus)
}
