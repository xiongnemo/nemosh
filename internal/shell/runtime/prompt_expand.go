package runtime

import (
	"context"
	"strings"
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
// -- the prompt's own backslash escapes are rendered afterwards, by the caller.
//
// Expansion happens before those escapes are rendered, deliberately. The other
// order would feed a directory name straight back into the parser, so a
// directory called `$(...)` would run it.
func (r Runtime) ExpandPromptString(ctx context.Context, text string, lastStatus int) string {
	if text == "" {
		return ""
	}
	tokens, err := scanShellTokens(`"` + escapeForPromptQuoting(text) + `"`)
	if err != nil || len(tokens) != 1 || tokens[0].parsed == nil {
		// An unbalanced quote or substitution is the user's, and a prompt is a
		// bad place to fail: show the text as written rather than nothing.
		return text
	}
	return strings.Join(r.expandWord(ctx, parseTypedWord(*tokens[0].parsed), lastStatus), "")
}

// escapeForPromptQuoting protects the quote that would end the wrapper, and
// nothing else -- $, backtick and backslash all have to keep meaning what they
// mean inside double quotes.
func escapeForPromptQuoting(text string) string {
	var escaped strings.Builder
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '"':
			escaped.WriteString(`\"`)
		case '\\':
			// A backslash pair is already the user's escape; pass both through
			// so `\\` does not become an escape for the character after it.
			escaped.WriteByte('\\')
			if index+1 < len(text) {
				index++
				escaped.WriteByte(text[index])
			}
		default:
			escaped.WriteByte(text[index])
		}
	}
	return escaped.String()
}
