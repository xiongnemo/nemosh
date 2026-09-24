package runtime

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// wait is POSIX `wait`, plus bash's `-n` (wait_next.go).
//
// With no operands it waits for every job and answers 0. With operands it waits for each in
// turn and answers the last one's status: `wait $a $b` is 4 in both references when b exits
// 4. It used to take exactly one operand, and refuse two with status 2.
//
// **An operand this shell does not know is 127.** POSIX says so for a process id -- one the
// shell does not know is treated as a process that exited 127 -- and both references answer
// `wait 99999` that way. For a job spec busybox says 2 and bash 127, and here the choice is
// made by `$!`: it *is* a job spec, so `wait $!` has to answer what `wait <pid>` answers
// everywhere else. That includes a job whose end `jobs` has already reported, which the shell
// no longer knows (POSIX 2.9.3). This used to be 2, "unknown or busy job", for both.
func (r Runtime) wait(ctx context.Context, args []string) int {
	if len(args) > 0 && args[0] == "-n" {
		return r.waitNext(ctx, args[1:])
	}
	if len(args) == 0 {
		return r.waitAll(ctx)
	}
	status := 0
	for _, operand := range args {
		status = r.waitOperand(ctx, operand)
		if ctx.Err() != nil {
			return status
		}
	}
	return status
}

func (r Runtime) waitAll(ctx context.Context) int {
	records, ok := r.jobScope.claimAll()
	if !ok {
		fmt.Fprintln(r.streams.Stderr, "wait: job is already being waited for")
		return 2
	}
	if _, err := waitJobs(ctx, records); err != nil {
		r.jobScope.releaseAll(records)
		return contextStatus(ctx)
	}
	r.reportSignalled(records)
	r.disposeCoprocs(records)
	r.jobScope.consumeAll(records)
	return 0
}

// waitOperand waits for one `%N` or process id, and answers its status.
func (r Runtime) waitOperand(ctx context.Context, operand string) int {
	id, status := r.waitTarget(operand)
	if status != 0 {
		return status
	}
	record, claimed := r.jobScope.claim(id)
	if !claimed {
		return r.unclaimable(operand, id)
	}
	status, err := waitJob(ctx, record)
	if err != nil {
		r.jobScope.release(record)
		return contextStatus(ctx)
	}
	r.reportSignalled([]*jobRecord{record})
	r.disposeCoprocs([]*jobRecord{record})
	if !r.jobScope.consumeAll([]*jobRecord{record}) {
		return 2
	}
	return status
}

// waitTarget reads an operand as a job: its id, or the status to answer instead.
func (r Runtime) waitTarget(operand string) (jobID, int) {
	if !strings.HasPrefix(operand, "%") {
		if pid, err := strconv.Atoi(operand); err == nil {
			// A job that is a process answers to its pid (job_process.go).
			if id, found := r.jobScope.lookupPID(pid); found {
				return id, 0
			}
			// Otherwise no number names a child of this shell. bash's words, and its
			// status.
			fmt.Fprintf(r.streams.Stderr, "wait: pid %s is not a child of this shell\n", operand)
			return 0, 127
		}
		fmt.Fprintf(r.streams.Stderr, "wait: `%s': not a pid or valid job spec\n", operand)
		return 0, 2
	}
	value, err := strconv.ParseUint(strings.TrimPrefix(operand, "%"), 10, 64)
	if err != nil || value == 0 {
		fmt.Fprintf(r.streams.Stderr, "wait: %s: invalid job\n", operand)
		return 0, 2
	}
	return jobID(value), 0
}

// unclaimable says why a job could not be waited for. Another `wait` holding it is a
// conflict, 2; not knowing it at all is the 127 above.
func (r Runtime) unclaimable(operand string, id jobID) int {
	if _, known := r.jobScope.lookup(id); known {
		fmt.Fprintf(r.streams.Stderr, "wait: %s: is already being waited for\n", operand)
		return 2
	}
	fmt.Fprintf(r.streams.Stderr, "wait: %s: no such job\n", operand)
	return 127
}
