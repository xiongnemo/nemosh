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
		segmentTokens, negated := stripPipelineNegation(segment.tokens)
		if len(segmentTokens) == 0 {
			return andOr{}, fmt.Errorf("syntax error: missing command after !")
		}
		commands, err := splitTokenPipeline(segmentTokens)
		if err != nil {
			if errors.Is(err, errPipelineMissingCommand) {
				return andOr{}, fmt.Errorf("%w: %v", ErrIncompleteScript, err)
			}
			return andOr{}, err
		}
		parsed := pipeline{negated: negated}
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

// stripPipelineNegation takes the `!` reserved word off the front of a pipeline.
// POSIX 2.9.2 gives it the whole pipeline, not the first command, so it comes
// off before the stages are split. Only one is recognised; a second `!` is an
// ordinary word, which is how dash reads it.
func stripPipelineNegation(tokens []shellToken) ([]shellToken, bool) {
	if len(tokens) == 0 || !isPipelineNegationToken(tokens[0]) {
		return tokens, false
	}
	return tokens[1:], true
}

// Quoting takes the reserved-word meaning away, so `"!" false` is a lookup for a
// command named `!` and has to reach command lookup unchanged.
func isPipelineNegationToken(token shellToken) bool {
	return token.kind == tokenWord && token.value == "!" && token.parsed != nil &&
		isUnquotedLiteralWord(*token.parsed)
}

// isUnquotedLiteralWord reports whether every part of a word is unquoted
// literal text. That is what makes a word eligible to be a reserved word or an
// alias: `'e'` and `"e"` are ordinary command names even where a bare `e` is
// not.
func isUnquotedLiteralWord(item word) bool {
	if len(item.parts) == 0 {
		return false
	}
	for _, part := range item.parts {
		if part.kind != wordPartLiteral || part.quote != quoteUnquoted {
			return false
		}
	}
	return true
}

func classifyCommandError(err error) error {
	if err == errMissingRedirectTarget || containsError(err, errMissingRedirectTarget) {
		return fmt.Errorf("%w: %v", ErrIncompleteScript, err)
	}
	return err
}
