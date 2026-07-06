package runtime

import (
	"context"
	"fmt"
	"strings"
)

type caseArm struct {
	pattern string
	body    []string
}

func (r Runtime) runCase(ctx context.Context, lines []string, start int) (int, int, flowControl) {
	word, err := r.caseWord(ctx, lines[start])
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "case: %v\n", err)
		return 2, len(lines), flowNone
	}
	esacIndex := caseEsacIndex(lines, start)
	if esacIndex < 0 {
		fmt.Fprintln(r.streams.Stderr, "case: missing esac")
		return 2, len(lines), flowNone
	}
	arms, err := parseCaseArms(lines[start+1 : esacIndex])
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "case: %v\n", err)
		return 2, esacIndex, flowNone
	}
	for _, arm := range arms {
		if arm.pattern != word && arm.pattern != "*" {
			continue
		}
		status, control := r.runLines(ctx, arm.body)
		return status, esacIndex, control
	}
	return 0, esacIndex, flowNone
}

func (r Runtime) caseWord(ctx context.Context, line string) (string, error) {
	args, err := splitWords(line)
	if err != nil {
		return "", err
	}
	if len(args) != 3 || args[0] != "case" || args[2] != "in" {
		return "", fmt.Errorf("expected: case word in")
	}
	return r.expandArg(ctx, args[1]), nil
}

func caseEsacIndex(lines []string, start int) int {
	for i := start + 1; i < len(lines); i++ {
		if lines[i] == "esac" {
			return i
		}
	}
	return -1
}

func parseCaseArms(lines []string) ([]caseArm, error) {
	var arms []caseArm
	for i := 0; i < len(lines); {
		pattern, ok := strings.CutSuffix(strings.TrimSpace(lines[i]), ")")
		if !ok || pattern == "" {
			return nil, fmt.Errorf("expected pattern)")
		}
		i++
		bodyStart := i
		for i < len(lines) && lines[i] != ";;" {
			i++
		}
		if i >= len(lines) {
			return nil, fmt.Errorf("missing ;;")
		}
		arms = append(arms, caseArm{pattern: pattern, body: lines[bodyStart:i]})
		i++
	}
	return arms, nil
}
