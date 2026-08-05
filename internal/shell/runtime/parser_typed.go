package runtime

import (
	"errors"
	"fmt"
)

func parseTypedScript(lines []string, spans []compoundSpan, budget *parseBudget, depth int) (Script, error) {
	byStart := indexCompounds(spans)
	program, err := parseTypedProgram(lines, spans, byStart, 0, len(lines), budget, depth)
	if err != nil {
		return Script{}, err
	}
	return Script{program: program}, nil
}

func indexCompounds(spans []compoundSpan) map[int]int {
	indexed := make(map[int]int, len(spans))
	for index, span := range spans {
		indexed[span.start] = index
	}
	return indexed
}

func parseTypedLine(line string) (list, error) {
	return parseTypedLineWithBudget(line, &parseBudget{}, 0)
}

func parseTypedLineWithBudget(line string, budget *parseBudget, depth int) (list, error) {
	masked, groups, err := extractGroupCommands(line, budget, depth)
	if err != nil {
		return list{}, err
	}
	if err := rejectDeferredSyntax(masked); err != nil {
		return list{}, err
	}
	tokens, err := scanExtractedGroups(masked, groups, budget, depth)
	if err != nil {
		return list{}, err
	}
	if len(tokens) == 0 {
		return list{}, nil
	}
	if tokens[0].kind == tokenPipe || tokens[0].kind == tokenAndIf || tokens[0].kind == tokenOrIf || tokens[0].kind == tokenBackground {
		return list{}, fmt.Errorf("syntax error: unexpected %s", tokens[0].value)
	}
	last := tokens[len(tokens)-1]
	if last.kind == tokenPipe || last.kind == tokenAndIf || last.kind == tokenOrIf {
		return list{}, fmt.Errorf("%w: missing command after %s", ErrIncompleteScript, last.value)
	}
	result := list{}
	start := 0
	for index, token := range tokens {
		if token.kind != tokenBackground {
			continue
		}
		if index == start {
			return list{}, fmt.Errorf("syntax error: unexpected %s", token.value)
		}
		parsed, err := parseAndOr(tokens[start:index], budget)
		if err != nil {
			return list{}, err
		}
		result.items = append(result.items, listItem{value: parsed, background: true})
		start = index + 1
	}
	if start < len(tokens) {
		parsed, err := parseAndOr(tokens[start:], budget)
		if err != nil {
			return list{}, err
		}
		result.items = append(result.items, listItem{value: parsed})
	}
	return result, nil
}

func parseAndOr(tokens []shellToken, budget *parseBudget) (andOr, error) {
	if len(tokens) == 0 {
		return andOr{}, fmt.Errorf("syntax error: missing command")
	}
	first := tokens[0]
	if first.kind == tokenPipe || first.kind == tokenAndIf || first.kind == tokenOrIf {
		return andOr{}, fmt.Errorf("syntax error: unexpected %s", first.value)
	}
	last := tokens[len(tokens)-1]
	if last.kind == tokenPipe || last.kind == tokenAndIf || last.kind == tokenOrIf {
		return andOr{}, fmt.Errorf("syntax error: missing command after %s", last.value)
	}
	for index := 1; index < len(tokens); index++ {
		if isAndOrOperator(tokens[index-1].kind) && isAndOrOperator(tokens[index].kind) {
			return andOr{}, fmt.Errorf("syntax error: unexpected %s", tokens[index].value)
		}
	}
	segments := splitTokenList(tokens)
	result := andOr{}
	pendingOperator := tokenWord
	for _, segment := range segments {
		if len(segment.tokens) == 0 {
			if segment.operator == tokenAndIf || segment.operator == tokenOrIf {
				pendingOperator = segment.operator
			}
			continue
		}
		commands, err := splitTokenPipeline(segment.tokens)
		if err != nil {
			if errors.Is(err, errPipelineMissingCommand) {
				return andOr{}, fmt.Errorf("%w: %v", ErrIncompleteScript, err)
			}
			return andOr{}, err
		}
		parsed := pipeline{}
		for _, commandTokens := range commands {
			command, redirects, err := parseRedirectsWithBudget(commandTokens, budget)
			if err != nil {
				return andOr{}, classifyCommandError(err)
			}
			if len(command) == 1 && command[0].group != nil {
				parsed.commands = append(parsed.commands, command[0].group.withRedirects(redirects))
				continue
			}
			words := make([]word, len(command))
			for index, token := range command {
				words[index] = parseTypedWord(*token.parsed)
			}
			parsed.commands = append(parsed.commands, simpleCommand{words: words, redirects: redirects})
		}
		result.pipelines = append(result.pipelines, parsed)
		if len(result.pipelines) > 1 {
			result.operators = append(result.operators, pendingOperator)
		}
	}
	return result, nil
}

func isAndOrOperator(kind tokenKind) bool {
	return kind == tokenAndIf || kind == tokenOrIf
}

func classifyCommandError(err error) error {
	if err == errMissingRedirectTarget || containsError(err, errMissingRedirectTarget) {
		return fmt.Errorf("%w: %v", ErrIncompleteScript, err)
	}
	return err
}
