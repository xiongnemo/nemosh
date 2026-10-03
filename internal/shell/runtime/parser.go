package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
)

var ErrIncompleteScript = errors.New("incomplete script")

func ParseScript(source string) (Script, error) {
	return parseScriptAt(source, 1)
}

func parseScript(source string, budget *parseBudget, depth int) (Script, error) {
	if depth > maxParseDepth {
		return Script{}, fmt.Errorf("command substitution depth: %w", errParseLimit)
	}
	if !budget.heredocsScanned {
		cleaned, heredocs, origins, ended, err := collectHeredocs(normalizeLineEndings(source), !budget.session)
		if err != nil {
			return Script{}, err
		}
		for _, heredoc := range ended {
			offset := budget.numbering.first - 1
			budget.warnings = append(budget.warnings, fmt.Sprintf("line %d: warning: here-document at line %d delimited by end-of-file (wanted `%s')",
				heredoc.lastLine+offset, heredoc.line+offset, visibleDelimiter(heredoc.delimiter)))
		}
		source = cleaned
		budget.numbering.origins = origins
		budget.heredocs = make(map[string]pendingHeredoc, len(heredocs))
		for _, heredoc := range heredocs {
			budget.heredocs[heredoc.marker] = heredoc
		}
		budget.heredocsScanned = true
	}
	// After heredocs are collected, so a quoted-delimiter body keeps its
	// backquotes as the literal text it is promised to be.
	source, err := rewriteBackquotes(source)
	if err != nil {
		return Script{}, err
	}
	source = quoteAssignmentSubscripts(source)
	lines, starts, breaks, err := numberedLogicalLines(source, budget.session && depth == 0)
	if err != nil {
		return Script{}, err
	}
	budget.numberLines(starts)
	budget.numbering.lines, budget.numbering.breaks = lines, breaks
	script, err := prepareScript(lines, budget, depth)
	if err == nil && depth == 0 {
		script.warnings = budget.warnings
	}
	return script, err
}

// prepareScript runs the passes that reshape lines, carrying where each one started
// (budget.numbering) through them.
func prepareScript(lines []string, budget *parseBudget, depth int) (Script, error) {
	at := budget.numbering.at
	before, beforeAt := lines, at
	lines, at = joinLinebreakIn(lines, at)
	lines, at = splitCompoundConditions(lines, at)
	lines, at = expandCaseArmLines(lines, at)
	lines, at = expandElifLines(lines, at)
	budget.numbering.at = at
	budget.numbering.breaks = carryBreaks(before, beforeAt, budget.numbering.breaks, lines, at)
	budget.numbering.lines = lines
	spans, err := compoundSpans(lines)
	if err != nil {
		return Script{}, err
	}
	return parseTypedScript(lines, spans, budget, depth)
}

func compoundSpans(lines []string) ([]compoundSpan, error) {
	var stack []compoundFrame
	var spans []compoundSpan
	for index, line := range lines {
		baseLine, background := trailingBackground(line)
		// A case terminator is not a command with a background `&` after it. Two of
		// the three spellings end in `&`, and stripping it turned `;&` into `;` --
		// which matched no case below, so the arm never closed and the next arm's
		// `)` arrived as a statement of its own. `;;&` only worked because the
		// terminator handed on was the whole line rather than this stripped one.
		if isCaseTerminator(line) {
			baseLine, background = line, false
		}
		if err := requireCaseBoundary(stack, baseLine); err != nil {
			return nil, err
		}
		kind, opener := compoundOpener(baseLine)
		pipelinePrefix, prefixOperator, compoundHeaderLine := "", "", ""
		if !opener {
			// `cmd | while ...`, `cmd && if ...`, `coproc NAME while ...`: a compound that
			// begins after something else on its line. Looked for only when the line does
			// not already begin with one, so the ordinary case pays nothing. See
			// parser_operator_compound.go.
			if prefix, operator, rest, ok := splitCompoundAfterPrefix(baseLine); ok {
				if kind, opener = compoundOpener(rest); opener {
					pipelinePrefix, prefixOperator, compoundHeaderLine = prefix, operator, rest
				}
			}
		}
		if opener {
			if len(stack) >= maxParseDepth {
				return nil, fmt.Errorf("compound depth: %w", errParseLimit)
			}
			// `done | while read l`: the words before the operator close the compound the new
			// one follows; see parser_compound_chain.go.
			chained, err := closeChainedCompound(&stack, &spans, pipelinePrefix, prefixOperator, index)
			if err != nil {
				return nil, err
			}
			if chained {
				pipelinePrefix = ""
			}
			stack = append(stack, compoundFrame{span: compoundSpan{
				kind: kind, start: index, thenIndex: -1, elseIndex: -1, doIndex: -1, afterCompound: chained,
				prefix: pipelinePrefix, prefixOperator: prefixOperator, header: compoundHeaderLine,
			}})
			continue
		}
		switch baseLine {
		case "then":
			if err := markThen(stack, index); err != nil {
				return nil, err
			}
		case "else":
			if err := markElse(stack, index); err != nil {
				return nil, err
			}
		case "do":
			if err := markDo(stack, index); err != nil {
				return nil, err
			}
		case ";;", ";;&", ";&":
			if err := closeCaseArm(stack, index, line); err != nil {
				return nil, err
			}
		case "fi", "done", "esac":
			closed, err := closeCompound(stack, baseLine, index)
			if err != nil {
				return nil, err
			}
			stack = stack[:len(stack)-1]
			closed.background = background
			spans = append(spans, closed)
		default:
			// `done < file`, `fi > log`, `esac | cat` -- a closer with something after
			// it. Recognised here rather than left to the default, which took it for
			// a command and reported the compound unterminated: `while read -r l; do
			// :; done < /dev/null` said `missing done`, and reading a file without a
			// subshell is what that form is for.
			if closer, suffix, ok := splitCompoundCloser(baseLine); ok {
				closed, err := closeCompound(stack, closer, index)
				if err != nil {
					return nil, err
				}
				stack = stack[:len(stack)-1]
				closed.background = background
				closed.suffix = suffix
				spans = append(spans, closed)
				continue
			}
			markCasePattern(stack, baseLine, index)
		}
	}
	if len(stack) != 0 {
		top := stack[len(stack)-1]
		closer := "fi"
		switch top.span.kind {
		case compoundLoop:
			closer = "done"
		case compoundCase:
			closer = "esac"
		}
		return nil, fmt.Errorf("%w: missing %s for compound at line %d", ErrIncompleteScript, closer, top.span.start+1)
	}
	return orderSpans(spans), nil
}

func compoundOpener(line string) (compoundKind, bool) {
	if kind, bare := bareConditionOpener(line); bare {
		return kind, true
	}
	switch {
	case hasCompoundHeader(line, "if"):
		return compoundIf, true
	case hasCompoundHeader(line, "for"), hasCompoundHeader(line, "while"), hasCompoundHeader(line, "until"),
		hasCompoundHeader(line, "select"):
		return compoundLoop, true
	case hasCompoundHeader(line, "case"):
		return compoundCase, true
	default:
		return 0, false
	}
}

func hasCompoundHeader(line string, keyword string) bool {
	_, ok := compoundHeader(line, keyword)
	return ok
}

func requireTop(stack []compoundFrame, want compoundKind, word string) error {
	if len(stack) == 0 || stack[len(stack)-1].span.kind != want {
		return fmt.Errorf("syntax error: unexpected %s", word)
	}
	return nil
}

func markDo(stack []compoundFrame, index int) error {
	if err := requireTop(stack, compoundLoop, "do"); err != nil {
		return err
	}
	if stack[len(stack)-1].span.doIndex >= 0 {
		return fmt.Errorf("syntax error: duplicate do")
	}
	stack[len(stack)-1].span.doIndex = index
	return nil
}

func markThen(stack []compoundFrame, index int) error {
	if err := requireTop(stack, compoundIf, "then"); err != nil {
		return err
	}
	if stack[len(stack)-1].span.thenIndex >= 0 {
		return fmt.Errorf("syntax error: duplicate then")
	}
	stack[len(stack)-1].span.thenIndex = index
	return nil
}

func markElse(stack []compoundFrame, index int) error {
	if err := requireTop(stack, compoundIf, "else"); err != nil {
		return err
	}
	top := &stack[len(stack)-1]
	if top.span.thenIndex < 0 || top.span.elseIndex >= 0 {
		return fmt.Errorf("syntax error: unexpected else")
	}
	top.span.elseIndex = index
	return nil
}

func closeCompound(stack []compoundFrame, word string, end int) (compoundSpan, error) {
	want := map[string]compoundKind{"fi": compoundIf, "done": compoundLoop, "esac": compoundCase}[word]
	if err := requireTop(stack, want, word); err != nil {
		return compoundSpan{}, err
	}
	top := stack[len(stack)-1]
	if top.span.kind == compoundIf && top.span.thenIndex < 0 {
		return compoundSpan{}, fmt.Errorf("syntax error: fi before then")
	}
	if top.span.kind == compoundLoop && top.span.doIndex < 0 {
		return compoundSpan{}, fmt.Errorf("syntax error: done before do")
	}
	if top.span.kind == compoundCase && top.casePatternSet {
		top.span.caseArms = append(top.span.caseArms, caseArmSpan{
			patternIndex: top.casePattern,
			bodyStart:    top.casePattern + 1,
			bodyEnd:      end,
		})
	}
	top.span.end = end
	return top.span, nil
}

func orderSpans(spans []compoundSpan) []compoundSpan {
	slices.SortFunc(spans, func(left, right compoundSpan) int { return left.start - right.start })
	return spans
}

// visibleDelimiter is a delimiter as a diagnostic shows it, a control byte in it written as
// an escape, so that a script cannot reach the terminal through the warning about it.
func visibleDelimiter(delimiter string) string {
	quoted := strconv.Quote(delimiter)
	return quoted[1 : len(quoted)-1]
}
