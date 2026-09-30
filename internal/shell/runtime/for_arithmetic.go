package runtime

import (
	"context"
	"fmt"
	"strings"
)

// `for ((init; condition; step))` -- the counted loop.
//
// It reported `expected: for name in words`, because the only `for` this parser
// knew was the word-list one. Together with `((expr))`, which parsed as nested
// subshells, that ruled out every loop anyone writes with a counter.
//
// The three parts are arithmetic expressions and go through the same evaluator
// `$(( ))` and `let` use. An empty part is not an error: `for ((;;))` is a loop
// forever, which is what an empty condition means, and an empty init or step is
// simply nothing to do.

// parseArithmeticForHeader reads the `((init; condition; step))` header, and reports
// whether that is what it was.
//
// The header must be *only* the parenthesised group. `for ((i=0;i<3;i++)) extra` is
// not a form bash has either, and accepting it would mean silently ignoring the
// extra word.
func parseArithmeticForHeader(header string) (arithmeticLoop, bool) {
	trimmed := strings.TrimSpace(header)
	end := arithmeticCommandEnd(trimmed, 0)
	if end == 0 || end != len(trimmed)-1 {
		return arithmeticLoop{}, false
	}
	parts := splitArithmeticForParts(arithmeticCommandText(trimmed, 0, end))
	if len(parts) != 3 {
		return arithmeticLoop{}, false
	}
	// Blanks before each part are dropped and those after it kept, as bash keeps them: they
	// are in its trace, its $BASH_COMMAND and its `declare -f`, `((i++  ))`.
	return arithmeticLoop{
		initialize: strings.TrimLeft(parts[0], " \t"),
		condition:  strings.TrimLeft(parts[1], " \t"),
		step:       strings.TrimLeft(parts[2], " \t"),
	}, true
}

// splitArithmeticForParts cuts the header on its two top-level semicolons.
//
// Top-level, because a part may hold parentheses of its own: `for ((i=(a+b); ...))`
// is legal and its `(` must not swallow the separator. Nothing else can appear
// there -- no quotes, no expansions -- so the depth count is the whole of it.
func splitArithmeticForParts(text string) []string {
	var parts []string
	depth, start := 0, 0
	for index := 0; index < len(text); index++ {
		switch text[index] {
		case '(':
			depth++
		case ')':
			depth--
		case ';':
			if depth == 0 {
				parts = append(parts, text[start:index])
				start = index + 1
			}
		}
	}
	return append(parts, text[start:])
}

// executeArithmeticFor runs the loop.
//
// The step runs after the body on every iteration including one cut short by
// `continue`, which is what C does and what stops `for ((i=0;i<3;i++)); do
// continue; done` from spinning forever.
func (r Runtime) executeArithmeticFor(ctx context.Context, node loopNode, savedStatus int) lineResult {
	r.loops.enter()
	defer r.loops.leave()
	// Each of the three parts is a command of its own to the DEBUG trap, on the loop's line.
	if result, ended := r.debugTrapHead(ctx, node.line, arithmeticHead(node.arith.initialize), savedStatus); ended {
		return result
	}
	if _, err := r.evaluateArithmetic(r.arithmeticForPart(ctx, node.arith.initialize, savedStatus)); err != nil {
		return r.arithmeticForFailure(err)
	}
	status := 0
	for {
		if ctx.Err() != nil {
			return lineResult{status: contextStatus(ctx)}
		}
		// The condition and the step are on the loop's line, and so is their $LINENO, as in
		// bash: `for ((i = 0; i < $LINENO; i++))` counted to the line of the body's last command.
		r.enterLine(node.line)
		if result, ended := r.debugTrapHead(ctx, node.line, arithmeticHead(node.arith.condition), savedStatus); ended {
			return result
		}
		keepGoing, err := r.arithmeticLoopCondition(r.arithmeticForPart(ctx, node.arith.condition, savedStatus))
		if err != nil {
			return r.arithmeticForFailure(err)
		}
		if !keepGoing {
			return lineResult{status: status}
		}
		bodyStatus, control := r.executeProgram(ctx, node.body, savedStatus)
		status, savedStatus = bodyStatus, bodyStatus
		if ctx.Err() != nil {
			return lineResult{status: contextStatus(ctx)}
		}
		switch control {
		case flowNone:
		case flowContinue:
			if !r.loops.consume() {
				return lineResult{status: 0, control: flowContinue}
			}
			status, savedStatus = 0, 0
		case flowBreak:
			if !r.loops.consume() {
				return lineResult{status: 0, control: flowBreak}
			}
			return lineResult{status: 0}
		default:
			return lineResult{status: status, control: control}
		}
		r.enterLine(node.line)
		if result, ended := r.debugTrapHead(ctx, node.line, arithmeticHead(node.arith.step), savedStatus); ended {
			return result
		}
		if _, err := r.evaluateArithmetic(r.arithmeticForPart(ctx, node.arith.step, savedStatus)); err != nil {
			return r.arithmeticForFailure(err)
		}
	}
}

// arithmeticForPart is one of the loop's parts as it is evaluated: expanded, an empty one
// being 1, as bash has it, and traced under `set -x` as `(( ))` is.
func (r Runtime) arithmeticForPart(ctx context.Context, part string, savedStatus int) string {
	expanded := "1"
	if part != "" {
		expanded = r.expandArithmeticText(ctx, part, savedStatus)
	}
	r.traceArithmetic(ctx, expanded, savedStatus)
	return expanded
}

// arithmeticForFailure ends the loop over an error in its header. A counter that
// is readonly has been reported already, by assignVar, and is a shell error.
func (r Runtime) arithmeticForFailure(err error) lineResult {
	if r.shellErrorRaised() {
		return r.shellErrorResult()
	}
	fmt.Fprintf(r.streams.Stderr, "for: %v\n", err)
	return lineResult{status: 1}
}

// arithmeticLoopCondition evaluates the middle part. An empty one is true, which is
// what makes `for ((;;))` a loop forever rather than a loop that never runs.
func (r Runtime) arithmeticLoopCondition(expression string) (bool, error) {
	if expression == "" {
		return true, nil
	}
	value, err := r.evaluateArithmetic(expression)
	if err != nil {
		return false, err
	}
	return value != 0, nil
}
