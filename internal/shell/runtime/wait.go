package runtime

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func (r Runtime) wait(ctx context.Context, args []string) int {
	if len(args) == 0 {
		records, ok := r.jobScope.claimAll()
		if !ok {
			fmt.Fprintln(r.streams.Stderr, "wait: job is already being waited for")
			return 2
		}
		if _, err := waitJobs(ctx, records); err != nil {
			r.jobScope.releaseAll(records)
			return contextStatus(ctx)
		}
		r.jobScope.consumeAll(records)
		return 0
	}
	if len(args) != 1 || !strings.HasPrefix(args[0], "%") {
		fmt.Fprintln(r.streams.Stderr, "wait: expected zero operands or one %N operand")
		return 2
	}
	value, err := strconv.ParseUint(strings.TrimPrefix(args[0], "%"), 10, 64)
	if err != nil || value == 0 {
		fmt.Fprintln(r.streams.Stderr, "wait: invalid job")
		return 2
	}
	record, ok := r.jobScope.claim(jobID(value))
	if !ok {
		fmt.Fprintln(r.streams.Stderr, "wait: unknown or busy job")
		return 2
	}
	status, err := waitJob(ctx, record)
	if err != nil {
		r.jobScope.release(record)
		return contextStatus(ctx)
	}
	if !r.jobScope.consumeAll([]*jobRecord{record}) {
		return 2
	}
	return status
}
