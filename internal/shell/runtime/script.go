package runtime

import (
	"bufio"
	"context"
	"fmt"
	"strings"
)

func (r Runtime) RunScript(ctx context.Context, script string) int {
	lines, err := scriptLines(script)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return 2
	}
	status, stop := r.runLines(ctx, lines)
	if stop {
		return status
	}
	return status
}

func scriptLines(script string) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(normalizeCRLF(script)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}

func (r Runtime) runLines(ctx context.Context, lines []string) (int, bool) {
	status := 0
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "if ") {
			ifStatus, next, stop := r.runIf(ctx, lines, i)
			status = ifStatus
			if stop {
				return status, true
			}
			i = next
			continue
		}
		result := r.runLine(ctx, line)
		status = result.status
		if result.stop {
			return status, true
		}
	}
	return status, false
}

func (r Runtime) runIf(ctx context.Context, lines []string, start int) (int, int, bool) {
	thenIndex := -1
	elseIndex := -1
	fiIndex := -1
	for i := start + 1; i < len(lines); i++ {
		switch lines[i] {
		case "then":
			thenIndex = i
		case "else":
			elseIndex = i
		case "fi":
			fiIndex = i
		}
		if fiIndex >= 0 {
			break
		}
	}
	if thenIndex < 0 || fiIndex < 0 {
		fmt.Fprintln(r.streams.Stderr, "if: missing then or fi")
		return 2, len(lines), false
	}
	condition := strings.TrimSpace(strings.TrimPrefix(lines[start], "if "))
	conditionResult := r.runLine(ctx, condition)
	if conditionResult.stop {
		return conditionResult.status, fiIndex, true
	}
	branchStart := thenIndex + 1
	branchEnd := fiIndex
	if conditionResult.status != 0 {
		if elseIndex < 0 {
			return 0, fiIndex, false
		}
		branchStart = elseIndex + 1
	} else if elseIndex >= 0 {
		branchEnd = elseIndex
	}
	status, stop := r.runLines(ctx, lines[branchStart:branchEnd])
	return status, fiIndex, stop
}
