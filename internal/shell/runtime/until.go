package runtime

import (
	"context"
	"fmt"
	"strings"
)

func (r Runtime) runUntil(ctx context.Context, lines []string, start int) (int, int, flowControl) {
	doIndex, doneIndex := doDoneIndexes(lines, start)
	if doIndex < 0 || doneIndex < 0 {
		fmt.Fprintln(r.streams.Stderr, "until: missing do or done")
		return 2, len(lines), flowNone
	}
	condition := strings.TrimSpace(strings.TrimPrefix(lines[start], "until "))
	status := 0
	for {
		conditionResult := r.runLine(ctx, condition)
		if conditionResult.control != flowNone {
			return conditionResult.status, doneIndex, conditionResult.control
		}
		if conditionResult.status == 0 {
			return status, doneIndex, flowNone
		}
		bodyStatus, control := r.runLines(ctx, lines[doIndex+1:doneIndex])
		status = bodyStatus
		if control == flowBreak {
			return 0, doneIndex, flowNone
		}
		if control == flowContinue {
			status = 0
			continue
		}
		if control != flowNone {
			return status, doneIndex, control
		}
	}
}
