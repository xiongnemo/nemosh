package runtime

import (
	"fmt"
	"strconv"
)

func (r Runtime) jobs(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(r.streams.Stderr, "jobs: expected no operands")
		return 2
	}
	// Reported is consumed: POSIX 2.9.3 removes a job from the list once the shell has
	// reported its status, and busybox agrees -- `[1]+ Done` appears once and `jobs`
	// afterwards says nothing. This used to answer `[1] Done(1)` every time it was asked.
	var reported []*jobRecord
	for _, record := range r.jobScope.snapshot() {
		state := "Running"
		select {
		case <-record.done:
			state = "Done"
			if record.status != 0 {
				state += "(" + strconv.Itoa(record.status) + ")"
			}
			reported = append(reported, record)
		default:
		}
		line := fmt.Sprintf("[%d] %s\n", record.id, state)
		if _, err := r.streams.Stdout.Write([]byte(line)); err != nil {
			fmt.Fprintf(r.streams.Stderr, "jobs: %v\n", err)
			// Forgotten anyway: the status reached the caller's stream or it did not,
			// and either way saying it again on the next ask is not the repair.
			r.jobScope.forget(reported)
			return 1
		}
	}
	r.jobScope.forget(reported)
	return 0
}
