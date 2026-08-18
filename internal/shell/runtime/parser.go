package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

var ErrIncompleteScript = errors.New("incomplete script")

type compoundKind uint8

const (
	compoundIf compoundKind = iota
	compoundLoop
	compoundCase
)

// suffix is what followed the closer -- a redirection, or a pipe into another
// command. Empty for the ordinary case.
type compoundSpan struct {
	kind       compoundKind
	background bool
	start      int
	thenIndex  int
	elseIndex  int
	doIndex    int
	end        int
	caseArms   []caseArmSpan
	// suffix is what followed the closer -- a redirection, or a pipe into another
	// command. Empty for the ordinary case; see splitCompoundCloser.
	suffix string
}

// splitCompoundCloser reads a closer with a suffix: `done < file` gives `done` and
// `< file`.
//
// The suffix has to begin with a redirection or a pipe. `done extra` is not a closer
// with a suffix, it is a syntax error, and calling it one here would hide the mistake.
func splitCompoundCloser(line string) (string, string, bool) {
	for _, closer := range [...]string{"fi", "done", "esac"} {
		rest, ok := strings.CutPrefix(line, closer)
		if !ok || rest == "" {
			continue
		}
		if rest[0] != ' ' && rest[0] != '	' && rest[0] != '<' && rest[0] != '>' {
			continue
		}
		suffix := strings.TrimSpace(rest)
		// A redirection only. A pipe after a closer is left to fail as it did, because
		// handling it needs the pipeline built from the remaining words and the brace
		// group spelling already works; see wrapCompoundWithSuffix.
		if suffix == "" || !strings.ContainsAny(suffix[:1], "<>") {
			continue
		}
		return closer, suffix, true
	}
	return "", "", false
}

type caseArmSpan struct {
	patternIndex int
	bodyStart    int
	bodyEnd      int
	// terminator is `;;`, `;;&` or `;&`, which decides what happens after the body
	// runs. See caseArmNode.
	terminator string
}

type compoundFrame struct {
	span           compoundSpan
	casePattern    int
	casePatternSet bool
}

func ParseScript(source string) (Script, error) {
	if len(source) > maxParseInputBytes {
		return Script{}, fmt.Errorf("input bytes: %w", errParseLimit)
	}
	return parseScript(source, &parseBudget{}, 0)
}

func parseScript(source string, budget *parseBudget, depth int) (Script, error) {
	if depth > maxParseDepth {
		return Script{}, fmt.Errorf("command substitution depth: %w", errParseLimit)
	}
	if !budget.heredocsScanned {
		cleaned, heredocs, err := collectHeredocs(normalizeLineEndings(source))
		if err != nil {
			return Script{}, err
		}
		source = cleaned
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
	lines, err := logicalLines(source)
	if err != nil {
		return Script{}, err
	}
	return prepareScript(lines, budget, depth)
}

func prepareScript(lines []string, budget *parseBudget, depth int) (Script, error) {
	lines = expandElifLines(expandCaseArmLines(lines))
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
		if opener {
			if len(stack) >= maxParseDepth {
				return nil, fmt.Errorf("compound depth: %w", errParseLimit)
			}
			stack = append(stack, compoundFrame{span: compoundSpan{
				kind: kind, start: index, thenIndex: -1, elseIndex: -1, doIndex: -1,
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
	switch {
	case hasCompoundHeader(line, "if"):
		return compoundIf, true
	case hasCompoundHeader(line, "for"), hasCompoundHeader(line, "while"), hasCompoundHeader(line, "until"):
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
