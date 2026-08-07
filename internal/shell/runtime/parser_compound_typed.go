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

func parseTypedFor(header string, body []programNode, budget *parseBudget, depth int) (programNode, error) {
	tokens, err := scanShellTokensWithBudget(header, budget, depth)
	if err != nil {
		return nil, err
	}
	if len(tokens) < 3 || tokens[1].value != "in" {
		return nil, fmt.Errorf("expected: for name in words")
	}
	values := make([]word, len(tokens)-2)
	for index, token := range tokens[2:] {
		values[index] = parseTypedWord(*token.parsed)
	}
	return loopNode{kind: loopFor, name: tokens[0].value, values: values, body: body}, nil
}

func parseTypedCase(lines []string, spans []compoundSpan, byStart map[int]int, span compoundSpan, budget *parseBudget, depth int) (programNode, error) {
	tokens, err := scanShellTokensWithBudget(lines[span.start], budget, depth)
	if err != nil || len(tokens) != 3 {
		return nil, fmt.Errorf("case: expected: case word in")
	}
	node := caseNode{word: parseTypedWord(*tokens[1].parsed)}
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
		node.arms = append(node.arms, caseArmNode{patterns: patterns, body: body})
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
		patterns = append(patterns, parseTypedWord(*tokens[0].parsed))
	}
	return patterns, nil
}
