package runtime

import (
	"context"
	"strings"
)

// `set -x` for the commands that are not a command name and its words: `(( ))` and the parts
// of an arithmetic `for`, `[[ ]]`, and the array assignments. Each was run without a trace,
// or traced as what it runs as: `(( a = 42 ))` came out as `let ' a = 42 '`.

// traceArithmetic is `(( expression ))` as bash traces it: the expression expanded, with a
// blank between it and each pair of parentheses, so `(( a = 42 ))` is `((  a = 42  ))`.
// busybox has no `(( ))`.
func (r Runtime) traceArithmetic(ctx context.Context, expression string, savedStatus int) {
	r.traceLine(ctx, "(( "+expression+" ))", savedStatus)
}

// traceConditionTest is one test of a `[[ ]]` as bash traces it, as the expression comes to
// it: `+ [[ 5 == 5 ]]`, then `+ [[ -n 'a b' ]]`, a lone word with its `-n`, and `!` before a
// test it negates. The whole command was traced once, as busybox's builtin is, its words
// expanded first; they are expanded as they are wanted now, so a test that is never made is
// not traced either. Each operand is quoted as any traced word is.
func (r Runtime) traceConditionTest(ctx context.Context, node *conditionNode, operands []string, savedStatus int) {
	if !r.options.xtrace {
		return
	}
	traced := []string{"[["}
	if node.negated {
		traced = append(traced, "!")
	}
	quoted := make([]string, len(operands))
	for index, operand := range operands {
		quoted[index] = traceWord(operand)
	}
	if node.kind == conditionUnary {
		traced = append(traced, node.operator, quoted[0])
	} else {
		traced = append(traced, quoted[0], node.operator, quoted[1])
	}
	r.traceLine(ctx, strings.Join(append(traced, "]]"), " "), savedStatus)
}

// traceArrayAssignment is an array assignment as bash traces it, busybox having no arrays: a
// list as written, its words one blank apart, `a=(1 "2 3")`; an element with its value
// expanded and quoted as an assignment's is, and its subscript as written, `a[$i]='x y'`.
func (r Runtime) traceArrayAssignment(ctx context.Context, assignment arrayAssignment, value string, savedStatus int) {
	if !r.options.xtrace {
		return
	}
	target := assignment.name
	if assignment.subscript != "" {
		target += "[" + assignment.subscript + "]"
	}
	if assignment.append {
		target += "+"
	}
	if !assignment.list {
		if value != "" {
			value = traceWord(value)
		}
		r.traceLine(ctx, target+"="+value, savedStatus)
		return
	}
	var words []string
	if tokens, err := scanShellTokens(strings.TrimSpace(assignment.raw)); err == nil {
		for _, token := range tokens {
			if token.kind == tokenWord && token.parsed != nil {
				words = append(words, printWord(*token.parsed))
			}
		}
	}
	r.traceLine(ctx, target+"=("+strings.Join(words, " ")+")", savedStatus)
}
