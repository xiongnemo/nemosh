package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

// jobLine is the one place a job's state is put into words.
//
// `jobs` and the notice before a prompt are two ways of asking the same question about the
// same record, and a notice that called a job something `jobs` did not would be two
// accounts of one thing. The bool says whether this is final -- which is also what decides
// whether reporting it consumes the job.
func jobLine(record *jobRecord) (string, bool) {
	select {
	case <-record.done:
		state := "Done"
		if record.status != 0 {
			state += "(" + strconv.Itoa(record.status) + ")"
		}
		return fmt.Sprintf("[%d] %s\n", record.id, state), true
	default:
		return fmt.Sprintf("[%d] Running\n", record.id), false
	}
}

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
		line, finished := jobLine(record)
		if finished {
			reported = append(reported, record)
		}
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

// ReportFinishedJobs names the jobs that have ended, and forgets them.
//
// Called by the interactive loops before they draw a prompt, which is when bash and
// busybox report it too. Neither needs `set -b` for this: `set -b` asks for the report
// *immediately*, in the middle of whatever is running, and that is the part this shell has
// no channel for and goes on refusing. A prompt is a moment the loop already has.
//
// On stderr, where the launch announcement goes and where busybox puts this one -- so a
// job ending while `x=$(...)` runs cannot end up inside the captured value.
//
// Nothing is written when there is nothing to say, so the common prompt costs one lock and
// no output.
func (r Runtime) ReportFinishedJobs() {
	var reported []*jobRecord
	var notice strings.Builder
	for _, record := range r.jobScope.snapshot() {
		line, finished := jobLine(record)
		if !finished {
			continue
		}
		notice.WriteString(line)
		reported = append(reported, record)
	}
	if len(reported) == 0 {
		return
	}
	fmt.Fprint(r.streams.Stderr, notice.String())
	r.jobScope.forget(reported)
}
