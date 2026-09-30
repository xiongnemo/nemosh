package runtime

import (
	"context"
	"strings"
	"time"
)

// SetVariable sets a shell variable from outside the runtime, which is how a
// startup file's effects and the interactive prompt's defaults reach it.
func (r Runtime) SetVariable(name, value string) {
	r.vars[name] = value
}

// ExpandPromptString expands PS1 or PS2 the way a prompt is expanded: parameter
// expansion, command substitution, and arithmetic, evaluated fresh every time
// the prompt is drawn. That is what lets a prompt show a git branch or the exit
// code of the command that just ran, and it is what dash, bash, and busybox ash
// all do.
//
// No field splitting and no pathname expansion: a prompt is one string, not a
// list of arguments, so `PS1='> '` must not lose its trailing space and a `*`
// in it must not turn into the contents of the directory.
//
// The text is scanned as though it were inside double quotes, which is what
// gives exactly those semantics. It also leaves `\033` and `\u` alone, since a
// backslash inside double quotes is only special before $, `, ", \ and newline
// -- the prompt's own backslash escapes are rendered afterwards; see Prompt.
// This is PS4's expansion as it stands, and $ENV's, which busybox expands as
// double-quoted text and no more.
//
// Expansion happens before those escapes are rendered, deliberately. The other
// order would feed a directory name straight back into the parser, so a
// directory called `$(...)` would run it.
func (r Runtime) ExpandPromptString(ctx context.Context, text string, lastStatus int) string {
	return r.expandPromptText(ctx, text, lastStatus, false)
}

// Prompt is PS1 or PS2 as it is drawn: expanded, and then its backslash escapes decoded, which
// is busybox's order. `\$` comes through the expansion as written for the decoding to make # or
// $ of it, as busybox's PSSYNTAX keeps it for PS1 and PS2; see decodePrompt for the escapes.
func (r Runtime) Prompt(ctx context.Context, text string, lastStatus int) string {
	return decodePrompt(r.expandPromptText(ctx, text, lastStatus, true), time.Now(), r.promptFacts(true), false)
}

// promptTransform is ${var@P}, bash's: the value decoded as a prompt is, with what each escape
// stands for quoted, and then expanded as double-quoted text -- the other order from the drawn
// prompt's, which bash has for both. So `x='\'; y=h; v='$x$y'` is \h, and not the host. It was
// refused, as a half of prompt expansion the line editor held.
func (r Runtime) promptTransform(ctx context.Context, value string, savedStatus int) string {
	return r.expandPromptText(ctx, decodePrompt(value, time.Now(), r.promptFacts(false), true), savedStatus, false)
}

// expandPromptText expands text as the inside of double quotes. keepEscapedDollar is busybox's
// PSSYNTAX, a prompt's: a `\$` comes out as written rather than as `$`.
func (r Runtime) expandPromptText(ctx context.Context, text string, lastStatus int, keepEscapedDollar bool) string {
	if text == "" {
		return ""
	}
	// Backquotes are rewritten to $(...) first, which is what parseScript does
	// for a script. A prompt does not go through parseScript, so without this a
	// PS1 of "`git branch`" showed its own backquotes instead of running.
	rewritten, err := rewriteBackquoteText(text, backquotesInDoubleQuotes)
	if err != nil {
		return text
	}
	tokens, err := scanShellTokens(`"` + escapeForPromptQuoting(rewritten, keepEscapedDollar) + `"`)
	if err != nil || len(tokens) != 1 || tokens[0].parsed == nil {
		// An unbalanced quote or substitution is the user's, and a prompt is a
		// bad place to fail: show the text as written rather than nothing.
		return text
	}
	return strings.Join(r.expandWord(ctx, parseTypedWord(*tokens[0].parsed), lastStatus), "")
}

// escapeForPromptQuoting protects the quote that would end the wrapper, and
// nothing else -- $, backtick and backslash all have to keep meaning what they
// mean inside double quotes. keepEscapedDollar makes `\$` one that comes out whole.
func escapeForPromptQuoting(text string, keepEscapedDollar bool) string {
	var escaped strings.Builder
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '"':
			escaped.WriteString(`\"`)
		case '\\':
			if keepEscapedDollar && index+1 < len(text) && text[index+1] == '$' {
				index++
				escaped.WriteString(`\\\$`)
				continue
			}
			// A backslash pair is already the user's escape; pass both through
			// so `\\` does not become an escape for the character after it. One
			// at the very end is a backslash, and not the closing quote's escape.
			escaped.WriteByte('\\')
			if index+1 == len(text) {
				escaped.WriteByte('\\')
			} else {
				index++
				escaped.WriteByte(text[index])
			}
		default:
			escaped.WriteByte(text[index])
		}
	}
	return escaped.String()
}
