package applets

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// ps lists processes.
//
// Nearly free, which is why it is here: `internal/proc` already lists them for
// `pgrep`, `pkill` and the `kill` builtin, so this is a formatter over a list
// three other things depend on. Doing it any other way would have meant a second
// CreateToolhelp32Snapshot, and two copies of a lookup drift.
//
// The columns are `PID` and `COMMAND`, which is the intersection of what every
// ps prints and what this can actually see. Notably absent: TTY, STAT, TIME and
// the full command line. Those are not omissions of effort -- Windows has no
// controlling terminal in the POSIX sense, and reading another process's command
// line means opening it and walking its PEB, which an ordinary session may not do
// for anything it does not own. A column of `?` per row would be worse than no
// column.
//
// busybox-w32's ps prints more than this by reading the snapshot's parent and
// thread fields; those are available and are not printed here for the same reason
// its own output is mostly `?` on Windows: a number nobody can act on is noise.
func newPsApplet() Applet {
	return simpleApplet{name: "ps", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		if _, _, err := parseAppletOptions(args, "", ""); err != nil {
			return err
		}
		processes, err := proc.List()
		if err != nil {
			// Reported, not answered with an empty list: "nothing is running" and
			// "I cannot see" are different, and only one of them is safe to act
			// on. The same rule pgrep follows.
			return err
		}
		sort.Slice(processes, func(i, j int) bool { return processes[i].PID < processes[j].PID })
		if _, err := fmt.Fprintf(stdout, "%7s %s\n", "PID", "COMMAND"); err != nil {
			return err
		}
		for _, process := range processes {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if _, err := fmt.Fprintf(stdout, "%7d %s\n", process.PID, process.Name); err != nil {
				return err
			}
		}
		return nil
	}}
}
