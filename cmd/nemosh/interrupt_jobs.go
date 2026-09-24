package main

import (
	"fmt"
	"io"
	"strings"
)

// reportEndedJobs says which background jobs Ctrl-C took with the script, and which of two
// answers that was. The references part here: busybox-w32 ends a script's jobs on Ctrl-C,
// and so does this (runtime.EndJobs); bash leaves them running, since POSIX has an
// asynchronous list ignore SIGINT. Someone used to either could be surprised by the other,
// so the choice is said out loud -- and only when there was a job to end, which keeps it
// from being the hint printed every time that nobody reads.
//
// On the shell's own stderr rather than the script's, which may have been redirected: this
// is for the person who pressed the key.
func reportEndedJobs(stderr io.Writer, ended []string) {
	switch len(ended) {
	case 0:
		return
	case 1:
		fmt.Fprintf(stderr, "nemosh: Ctrl-C ended the script, and the background job it had running: %s\n", ended[0])
	default:
		fmt.Fprintf(stderr, "nemosh: Ctrl-C ended the script, and the %d background jobs it had running: %s\n",
			len(ended), strings.Join(ended, ", "))
	}
	fmt.Fprintln(stderr, "hint: busybox ends a script's jobs on Ctrl-C too; bash would have left them running")
}
