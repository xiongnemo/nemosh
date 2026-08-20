package applets

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// ps lists processes.
//
// It printed two columns, PID and COMMAND, and a comment explained that TTY, STAT, TIME and the
// rest were absent because Windows either has not got them or hides them behind a handle this
// session may not open. Half of that was right and half was a limit of how the list was being
// read: CreateToolhelp32Snapshot answers with a name and a pid, so a name and a pid was all there
// was to print.
//
// The system table answers with rather more for no more privilege -- see internal/proc -- so the
// columns busybox-w32's ps prints are available now: `PID PPID USER TIME ELAPSED COMMAND` there,
// and `PID PPID THR RSS TIME COMMAND` here.
//
// Two of busybox's columns are still missing and the reasons differ. USER needs a handle on each
// process and is refused for anything this session does not own -- measured, 176 of 436 -- and a
// column that is right for a third of its rows is worse than no column. ELAPSED could be computed
// from the creation time, and is left out only because TIME is what people sort by.
//
// TTY and STAT remain absent for the original reason: Windows has no controlling terminal, and
// while a process state can be *derived* from its threads, `ps` is the wrong place to introduce an
// approximation. `top` shows it, where a monitor's reader expects one.
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
		if _, err := fmt.Fprintf(stdout, "%7s %7s %5s %9s %9s %s\n",
			"PID", "PPID", "THR", "RSS", "TIME", "COMMAND"); err != nil {
			return err
		}
		for _, process := range processes {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if _, err := fmt.Fprintf(stdout, "%7d %7d %5d %9s %9s %s\n",
				process.PID, process.PPID, process.Threads,
				humanBlocks(int64(process.WorkingSet+duBlock-1)/duBlock),
				processCPUTime(process.Kernel+process.User),
				process.Name); err != nil {
				return err
			}
		}
		return nil
	}}
}

// processCPUTime is cumulative CPU as `H:MM:SS`, which is what every ps prints and what a person
// reads to see which process has been busy since boot.
//
// Hours rather than days, because a process that has used more than a day of CPU is better shown
// as three digits of hours than in a second unit the column would have to widen for.
func processCPUTime(used time.Duration) string {
	hours := int(used / time.Hour)
	minutes := int(used/time.Minute) % 60
	seconds := int(used/time.Second) % 60
	return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
}
