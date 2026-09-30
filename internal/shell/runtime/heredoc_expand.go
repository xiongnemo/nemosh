package runtime

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// expandHeredocOperation is a heredoc's body expanded, and whether an expansion in it failed.
// That fails the redirection and not the script, as in both references: busybox expands the
// body as it makes the redirection, where an error is the redirection's, so `cat <<EOF` with
// `${D?}` in its body says so, is not run, and leaves 1, and the script goes on -- but for a
// special builtin's, which ends the script as any failed redirection of one does. The error
// ended every script, with 2.
func (r Runtime) expandHeredocOperation(ctx context.Context, body string, savedStatus int) (string, bool) {
	if r.expansion.shellError {
		return r.expandHeredocBody(ctx, body, savedStatus), false
	}
	expanded := r.expandHeredocBody(ctx, body, savedStatus)
	return expanded, r.shellErrorRaised()
}

// errHeredocBody is the redirection of a heredoc whose body did not expand, which has said why.
var errHeredocBody = errors.New("heredoc body did not expand")

func (r Runtime) expandHeredocBody(ctx context.Context, body string, savedStatus int) string {
	var expanded strings.Builder
	for index := 0; index < len(body); {
		if body[index] == '\\' && index+1 < len(body) && body[index+1] == '\n' {
			index += 2
			continue
		}
		if body[index] == '\\' && index+1 < len(body) && strings.IndexByte("$`\\", body[index+1]) >= 0 {
			expanded.WriteByte(body[index+1])
			index += 2
			continue
		}
		// A backquoted command, which POSIX expands in a heredoc as it does `$(...)`. The
		// backquote rewrite runs after the bodies are taken out, so it never reached one,
		// and `cat <<EOF` with `date` in backquotes printed the backquotes.
		// Its `\"` is a quote there as in double quotes, busybox's answer; bash keeps the
		// backslash.
		if body[index] == '`' {
			if inner, end, ok := backquoteBody(body, index, true); ok {
				if script, err := r.parseHere(inner); err == nil {
					expanded.WriteString(r.commandSubstitutionScript(ctx, script, savedStatus))
					index = end + 1
					continue
				}
			}
		}
		if body[index] != '$' {
			expanded.WriteByte(body[index])
			index++
			continue
		}
		// `$((...))` before `$(...)`, which took it for a command in a subshell: `$((1+1))`
		// ran a command named 1+1 and printed its not-found, where busybox prints 2.
		if strings.HasPrefix(body[index:], "$((") {
			if end, ok := arithmeticExpansionEnd(body, index+3); ok {
				value, err := r.evaluateArithmetic(r.expandArithmeticText(ctx, body[index+3:end-1], savedStatus))
				if err != nil {
					r.reportExpansionError(err)
				} else {
					expanded.WriteString(strconv.FormatInt(value, 10))
				}
				index = end + 1
				continue
			}
		}
		if index+1 < len(body) && body[index+1] == '(' {
			end, ok := commandSubstitutionEnd(body, index+2)
			if ok {
				script, err := r.parseHere(body[index+2 : end])
				if err == nil {
					expanded.WriteString(r.commandSubstitutionScript(ctx, script, savedStatus))
					index = end + 1
					continue
				}
			}
		}
		end := parameterEnd(body, index+1)
		if end > index+1 {
			values := r.expandParameterPart(ctx, wordPart{kind: wordPartParameter, text: body[index:end]}, savedStatus)
			expanded.WriteString(strings.Join(values, " "))
			index = end
			continue
		}
		expanded.WriteByte(body[index])
		index++
	}
	return expanded.String()
}
