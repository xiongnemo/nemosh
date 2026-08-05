package runtime

import (
	"fmt"
	"strings"
)

func markCasePattern(stack []compoundFrame, line string, index int) {
	if len(stack) == 0 || stack[len(stack)-1].span.kind != compoundCase {
		return
	}
	top := &stack[len(stack)-1]
	if top.casePatternSet {
		return
	}
	pattern, ok := casePattern(line)
	if !ok || pattern == "" {
		return
	}
	top.casePattern = index
	top.casePatternSet = true
}

func requireCaseBoundary(stack []compoundFrame, line string) error {
	if len(stack) == 0 {
		return nil
	}
	top := stack[len(stack)-1]
	if top.span.kind != compoundCase || top.casePatternSet || caseCloserOrContinuation(line) {
		return nil
	}
	pattern, ok := casePattern(line)
	if !ok || pattern == "" {
		return fmt.Errorf("case: expected pattern)")
	}
	return nil
}

func caseCloserOrContinuation(line string) bool {
	return line == "esac" || strings.HasPrefix(line, "esac &&") || strings.HasPrefix(line, "esac ||") || strings.HasPrefix(line, "esac |")
}

func casePattern(line string) (string, bool) {
	return strings.CutSuffix(strings.TrimSpace(line), ")")
}

func closeCaseArm(stack []compoundFrame, index int) error {
	if err := requireTop(stack, compoundCase, ";;"); err != nil {
		return err
	}
	top := &stack[len(stack)-1]
	if !top.casePatternSet {
		return fmt.Errorf("case: expected pattern)")
	}
	top.span.caseArms = append(top.span.caseArms, caseArmSpan{
		patternIndex: top.casePattern,
		bodyStart:    top.casePattern + 1,
		bodyEnd:      index,
	})
	top.casePatternSet = false
	return nil
}
