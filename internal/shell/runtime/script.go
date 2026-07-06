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
		if strings.HasPrefix(line, "for ") {
			forStatus, next, stop := r.runFor(ctx, lines, i)
			status = forStatus
			if stop {
				return status, true
			}
			i = next
			continue
		}
		if strings.HasPrefix(line, "while ") {
			whileStatus, next, stop := r.runWhile(ctx, lines, i)
			status = whileStatus
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

func (r Runtime) runWhile(ctx context.Context, lines []string, start int) (int, int, bool) {
	doIndex, doneIndex := doDoneIndexes(lines, start)
	if doIndex < 0 || doneIndex < 0 {
		fmt.Fprintln(r.streams.Stderr, "while: missing do or done")
		return 2, len(lines), false
	}
	condition := strings.TrimSpace(strings.TrimPrefix(lines[start], "while "))
	status := 0
	for {
		conditionResult := r.runLine(ctx, condition)
		if conditionResult.stop {
			return conditionResult.status, doneIndex, true
		}
		if conditionResult.status != 0 {
			return status, doneIndex, false
		}
		bodyStatus, stop := r.runLines(ctx, lines[doIndex+1:doneIndex])
		status = bodyStatus
		if stop {
			return status, doneIndex, true
		}
	}
}

func (r Runtime) runFor(ctx context.Context, lines []string, start int) (int, int, bool) {
	doIndex, doneIndex := doDoneIndexes(lines, start)
	if doIndex < 0 || doneIndex < 0 {
		fmt.Fprintln(r.streams.Stderr, "for: missing do or done")
		return 2, len(lines), false
	}
	name, values, err := r.forHeader(ctx, lines[start])
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "for: %v\n", err)
		return 2, doneIndex, false
	}
	status := 0
	for _, value := range values {
		r.vars[name] = value
		bodyStatus, stop := r.runLines(ctx, lines[doIndex+1:doneIndex])
		status = bodyStatus
		if stop {
			return status, doneIndex, true
		}
	}
	return status, doneIndex, false
}

func (r Runtime) forHeader(ctx context.Context, line string) (string, []string, error) {
	args, err := splitWords(strings.TrimSpace(strings.TrimPrefix(line, "for ")))
	if err != nil {
		return "", nil, err
	}
	args = r.expandArgs(ctx, args)
	if len(args) < 3 || args[1] != "in" {
		return "", nil, fmt.Errorf("expected: for name in words")
	}
	return args[0], args[2:], nil
}

func doDoneIndexes(lines []string, start int) (int, int) {
	doIndex := -1
	doneIndex := -1
	for i := start + 1; i < len(lines); i++ {
		switch lines[i] {
		case "do":
			doIndex = i
		case "done":
			doneIndex = i
		}
		if doneIndex >= 0 {
			break
		}
	}
	return doIndex, doneIndex
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
