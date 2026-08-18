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
	return arithmeticLoop{
		initialize: strings.TrimSpace(parts[0]),
		condition:  strings.TrimSpace(parts[1]),
		step:       strings.TrimSpace(parts[2]),
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
	if node.arith.initialize != "" {
		if _, err := r.evaluateArithmetic(r.expandArithmeticText(ctx, node.arith.initialize, savedStatus)); err != nil {
			fmt.Fprintf(r.streams.Stderr, "for: %v\n", err)
			return lineResult{status: 1}
		}
	}
	status := 0
	for {
		if ctx.Err() != nil {
			return lineResult{status: contextStatus(ctx)}
		}
		keepGoing, err := r.arithmeticLoopCondition(r.expandArithmeticText(ctx, node.arith.condition, savedStatus))
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "for: %v\n", err)
			return lineResult{status: 1}
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
		if node.arith.step != "" {
			if _, err := r.evaluateArithmetic(r.expandArithmeticText(ctx, node.arith.step, savedStatus)); err != nil {
				fmt.Fprintf(r.streams.Stderr, "for: %v\n", err)
				return lineResult{status: 1}
			}
		}
	}
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
