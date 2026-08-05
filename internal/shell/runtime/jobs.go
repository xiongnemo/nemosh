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
	for _, record := range r.jobScope.snapshot() {
		state := "Running"
		select {
		case <-record.done:
			state = "Done"
			if record.status != 0 {
				state += "(" + strconv.Itoa(record.status) + ")"
			}
		default:
		}
		line := fmt.Sprintf("[%d] %s\n", record.id, state)
		if _, err := r.streams.Stdout.Write([]byte(line)); err != nil {
			fmt.Fprintf(r.streams.Stderr, "jobs: %v\n", err)
			return 1
		}
	}
	return 0
}
