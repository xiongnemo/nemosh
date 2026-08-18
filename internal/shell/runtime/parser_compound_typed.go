package runtime

import "fmt"

func parseTypedProgram(lines []string, spans []compoundSpan, byStart map[int]int, start, end int, budget *parseBudget, depth int) ([]programNode, error) {
	var program []programNode
	for index := start; index < end; index++ {
		if spanIndex, ok := byStart[index]; ok {
			span := spans[spanIndex]
			node, err := parseTypedCompound(lines, spans, byStart, span, budget, depth)
			if err != nil {
				return nil, err
			}
			if span.suffix != "" {
				// A compound with a redirection or a pipe after it is exactly a brace
				// group holding that compound: same scope, same redirects, same
				// behaviour as a pipeline stage. Reusing the group rather than adding
				// a second thing that carries redirects is what keeps the two from
				// disagreeing -- `{ ...; } < file` already worked.
				if node, err = wrapCompoundWithSuffix(node, span.suffix, budget, depth); err != nil {
					return nil, err
				}
			}
			if span.background {
				node = backgroundNode{value: node}
			}
			program = append(program, node)
			index = span.end
			continue
		}
		line := lines[index]
		if standaloneFunctionHeader(line) {
			if index+1 >= end {
				return nil, fmt.Errorf("%w: missing function body", ErrIncompleteScript)
			}
			line += " " + lines[index+1]
			index++
		}
		definitionLine, background := trailingBackground(line)
		definition, recognized, err := parseFunctionDefinition(definitionLine, budget, depth)
		if err != nil {
			return nil, err
		}
		if recognized {
			var node programNode = definition
			if background {
				node = backgroundNode{value: node}
			}
			program = append(program, node)
			continue
		}
		parsed, err := parseTypedLineWithBudget(lines[index], budget, depth)
		if err != nil {
			return nil, err
		}
		if len(parsed.items) > 0 {
			program = append(program, listNode{value: parsed})
		}
	}
	return program, nil
}

func parseTypedCompound(lines []string, spans []compoundSpan, byStart map[int]int, span compoundSpan, budget *parseBudget, depth int) (programNode, error) {
	depth++
	if depth > maxParseDepth {
		return nil, fmt.Errorf("compound depth: %w", errParseLimit)
	}
	switch span.kind {
	case compoundIf:
		return parseTypedIf(lines, spans, byStart, span, budget, depth)
	case compoundLoop:
		return parseTypedLoop(lines, spans, byStart, span, budget, depth)
	case compoundCase:
		return parseTypedCase(lines, spans, byStart, span, budget, depth)
	default:
		return nil, fmt.Errorf("unknown compound kind %d", span.kind)
	}
}

func parseTypedIf(lines []string, spans []compoundSpan, byStart map[int]int, span compoundSpan, budget *parseBudget, depth int) (programNode, error) {
	header, _ := compoundHeader(lines[span.start], "if")
	condition, err := parseTypedLineWithBudget(header, budget, depth)
	if err != nil {
		return nil, err
	}
	thenEnd := span.end
	if span.elseIndex >= 0 {
		thenEnd = span.elseIndex
	}
	thenBody, err := parseTypedProgram(lines, spans, byStart, span.thenIndex+1, thenEnd, budget, depth)
	if err != nil {
		return nil, err
	}
	var elseBody []programNode
	if span.elseIndex >= 0 {
		elseBody, err = parseTypedProgram(lines, spans, byStart, span.elseIndex+1, span.end, budget, depth)
	}
	return ifNode{condition: condition, thenBody: thenBody, elseBody: elseBody}, err
}

func parseTypedLoop(lines []string, spans []compoundSpan, byStart map[int]int, span compoundSpan, budget *parseBudget, depth int) (programNode, error) {
	body, err := parseTypedProgram(lines, spans, byStart, span.doIndex+1, span.end, budget, depth)
	if err != nil {
		return nil, err
	}
	line := lines[span.start]
	if header, ok := compoundHeader(line, "for"); ok {
		return parseTypedFor(header, body, budget, depth)
	}
	kind, keyword := loopWhile, "while"
	if _, ok := compoundHeader(line, "until"); ok {
		kind, keyword = loopUntil, "until"
	}
	header, _ := compoundHeader(line, keyword)
	condition, err := parseTypedLineWithBudget(header, budget, depth)
	return loopNode{kind: kind, condition: condition, body: body}, err
}

// typedWordOf is the parsed form of a token, or a refusal naming it.
//
// Only a word token carries one: an operator such as `|` or `>` leaves parsed
// nil, and dereferencing that panicked the whole shell on input as ordinary as
// `for i in a|b; do :; done`. The fuzzer found it; the three places it could
// happen are a for-loop's word list, the word a case selects on, and a case
// arm's patterns. Everywhere else a redirect or a pipe is rejected before it
// gets this far.
//
// busybox answers these with `syntax error: unexpected "|"`; the wording here is
// this parser's own, which spells the same thing with %s.
func typedWordOf(token shellToken) (word, error) {
	if token.parsed == nil {
		return word{}, fmt.Errorf("syntax error: unexpected %s", token.value)
	}
	return parseTypedWord(*token.parsed), nil
}

func parseTypedFor(header string, body []programNode, budget *parseBudget, depth int) (programNode, error) {
	// The C-style form is settled before tokenizing, because the lexer rewrites a
	// leading `((expr))` into `let` -- correct for a command, wrong here, where the
	// three parts have to stay separate. See arithmetic_command.go.
	if loop, ok := parseArithmeticForHeader(header); ok {
		return loopNode{kind: loopArithmetic, arith: loop, body: body}, nil
	}
	tokens, err := scanShellTokensWithBudget(header, budget, depth)
	if err != nil {
		return nil, err
	}
	// `for name` with no list is `for name in "$@"`, which POSIX 2.9.4.2 specifies and
	// which is how a function loops over its arguments. It reported the usage instead.
	if len(tokens) == 1 && tokens[0].kind == tokenWord {
		return loopNode{kind: loopFor, name: tokens[0].value, overArguments: true, body: body}, nil
	}
	if len(tokens) < 2 || tokens[1].value != "in" {
		return nil, fmt.Errorf("expected: for name in words, or for ((init; condition; step))")
	}
	// `for name in` with nothing after it loops zero times. Not an error: it is what a
	// generated list that came out empty looks like.
	if len(tokens) == 2 {
		return loopNode{kind: loopFor, name: tokens[0].value, body: body}, nil
	}
	values := make([]word, len(tokens)-2)
	for index, token := range tokens[2:] {
		if values[index], err = typedWordOf(token); err != nil {
			return nil, err
		}
	}
	return loopNode{kind: loopFor, name: tokens[0].value, values: values, body: body}, nil
}

func parseTypedCase(lines []string, spans []compoundSpan, byStart map[int]int, span compoundSpan, budget *parseBudget, depth int) (programNode, error) {
	tokens, err := scanShellTokensWithBudget(lines[span.start], budget, depth)
	if err != nil || len(tokens) != 3 {
		return nil, fmt.Errorf("case: expected: case word in")
	}
	selector, err := typedWordOf(tokens[1])
	if err != nil {
		return nil, err
	}
	node := caseNode{word: selector}
	for _, arm := range span.caseArms {
		pattern, _ := casePattern(lines[arm.patternIndex])
		patterns, err := parseCaseAlternatives(pattern, budget, depth)
		if err != nil {
			return nil, err
		}
		body, err := parseTypedProgram(lines, spans, byStart, arm.bodyStart, arm.bodyEnd, budget, depth)
		if err != nil {
			return nil, err
		}
		node.arms = append(node.arms, caseArmNode{patterns: patterns, body: body, terminator: arm.terminator})
	}
	return node, nil
}

func parseCaseAlternatives(pattern string, budget *parseBudget, depth int) ([]word, error) {
	var patterns []word
	for _, alternative := range splitCaseAlternatives(pattern) {
		tokens, err := scanShellTokensWithBudget(alternative, budget, depth)
		if err != nil || len(tokens) != 1 {
			return nil, fmt.Errorf("case: invalid pattern %q", pattern)
		}
		typed, err := typedWordOf(tokens[0])
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, typed)
	}
	return patterns, nil
}

// wrapCompoundWithSuffix turns `while ...; done < file` into the brace group it is
// equivalent to.
//
// The suffix is parsed by handing the group's own machinery a line it already knows how
// to read: `{ compound; } suffix` cannot be built as text without re-quoting, so the
// group is built directly and only the suffix is parsed here.
func wrapCompoundWithSuffix(node programNode, suffix string, budget *parseBudget, depth int) (programNode, error) {
	tokens, err := scanShellTokensWithBudget(suffix, budget, depth)
	if err != nil {
		return nil, err
	}
	operations, err := parseRedirectsOnly(tokens)
	if err != nil {
		return nil, err
	}
	group := braceGroup{body: Script{program: []programNode{node}}, redirects: operations}
	item := listItem{value: andOr{pipelines: []pipeline{{commands: []commandNode{group}}}}}
	return listNode{value: list{items: []listItem{item}}}, nil
}
