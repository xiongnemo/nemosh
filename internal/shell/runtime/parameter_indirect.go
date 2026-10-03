package runtime

import (
	"context"
	"fmt"
	"strings"
)

// `${!ref}` is bash's indirection: the parameter whose name is ref's value. busybox has
// none of it. It is taken over whole rather than for the value alone, so the operator after
// it applies to the parameter ref names -- `${!ref-default}`, `${!ref#x}`, `${!ref:1:2}`,
// `${!ref@Q}` -- and ref may name any parameter: a variable, a positional parameter, `@`, an
// array element or a whole array. `${!#}`, the last positional parameter, is the commonest
// use of it there is.
//
// It stopped at the value, and at a variable's name: `${!#}` and every other target that was
// not a plain name was `invalid variable name`, and an operator after it was dropped, so
// `${!ref-default}` gave the empty string. And a ref that was not set gave the empty string
// where bash stops with `invalid indirect expansion`.

// reportIndirectError is an indirection that cannot be made -- ref not set, or its value no
// name -- which is bash's expand_wdesc_error: the command the shell was running is abandoned,
// the function it called and the loop it was in, status 1, and the shell goes on with the
// next, as failglob's error does; see shellErrorResult. It ended the script, status 2, as
// busybox's bad substitution does; busybox has no indirection.
func (r Runtime) reportIndirectError(err error) {
	if r.expansion.shellError {
		return
	}
	fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
	r.expansion.shellError, r.expansion.discard = true, true
}

// indirectText is `${!ref...}` rewritten as the expansion of the parameter ref names, and
// whether the text was an indirection at all. The text is empty when ref names nothing
// without that being an error; see indirectTarget.
func (r Runtime) indirectText(ctx context.Context, text string, savedStatus int) (string, bool, error) {
	body, ok := indirectBody(text)
	if !ok {
		return "", false, nil
	}
	target, tail, err := r.indirectTarget(ctx, body, savedStatus)
	if err != nil || target == "" {
		return "", true, err
	}
	return "${" + target + tail + "}", true, nil
}

// indirectBody is what follows the `!` of an indirection: not `${!}`, which is `$!`, nor
// `${!name[@]}` and `${!prefix@}`, which list an array's keys and the names with a prefix.
func indirectBody(text string) (string, bool) {
	body, ok := strings.CutPrefix(text, "${!")
	if !ok {
		return "", false
	}
	body, ok = strings.CutSuffix(body, "}")
	if !ok || body == "" {
		return "", false
	}
	if reference, isElement := parseArrayReference(body); isElement && (reference.subscript == "@" || reference.subscript == "*") {
		return "", false
	}
	for _, list := range []string{"@", "*"} {
		if prefix, found := strings.CutSuffix(body, list); found && isVariableName(prefix) {
			return "", false
		}
	}
	return body, true
}

// indirectTarget reads the reference an indirection's body begins with, and answers the
// parameter its value names and the operator after it.
//
// An array whose element zero is not set names nothing, and that is not an error: bash gives
// `${!A}` of a map with no key 0 as empty and goes on. Any other ref that is not set is.
func (r Runtime) indirectTarget(ctx context.Context, body string, savedStatus int) (string, string, error) {
	end := referenceEnd(body)
	if end == 0 {
		return "", "", fmt.Errorf("bad substitution: ${!%s}", body)
	}
	reference, tail := body[:end], body[end:]
	target, set := r.operandParameter(ctx, reference, savedStatus)
	if !set {
		if r.arrays.has(reference) || r.arrays.isAssociative(reference) {
			return "", tail, nil
		}
		return "", "", fmt.Errorf("%s: invalid indirect expansion", reference)
	}
	if !isParameterName(target) {
		return "", "", fmt.Errorf("%s: invalid variable name", target)
	}
	return target, tail, nil
}

// referenceEnd is the length of the parameter reference body begins with: a name and any
// subscript, a positional parameter's digits, or one special parameter.
func referenceEnd(body string) int {
	switch char := body[0]; {
	case char >= '0' && char <= '9':
		end := 1
		for end < len(body) && body[end] >= '0' && body[end] <= '9' {
			end++
		}
		return end
	case strings.IndexByte("#?$!-@*", char) >= 0:
		return 1
	case !isNameByte(char):
		return 0
	}
	end := 1
	for end < len(body) && isNameByte(body[end]) {
		end++
	}
	if end < len(body) && body[end] == '[' {
		if close := matchingBracket(body, end); close > end {
			end = close + 1
		}
	}
	return end
}

// isParameterName reports whether an indirection's target names a parameter: a variable, an
// element or all of an array, a positional parameter, or a special one.
func isParameterName(target string) bool {
	if isVariableName(target) || isArrayElementTarget(target) {
		return true
	}
	if target != "" && strings.Trim(target, "0123456789") == "" {
		return true
	}
	return len(target) == 1 && strings.IndexByte("@*#?-$!", target[0]) >= 0
}
