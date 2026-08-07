package runtime

import (
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// times reports the CPU the shell and its finished children have used, in the
// two lines and the `%dm%fs` shape POSIX 2.14 specifies: the shell's user and
// system time first, then the children's.
//
// The children's half is accumulated as each external process is waited for,
// because that is the only moment Go reports it. A child that was cancelled or
// never started contributes nothing, which is also true of the shell it models.
func (r Runtime) times() int {
	selfUser, selfSystem, err := processCPUTime()
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "times: %v\n", err)
		return 1
	}
	childUser, childSystem := r.childCPU.total()
	fmt.Fprintf(r.streams.Stdout, "%s %s\n", formatCPUTime(selfUser), formatCPUTime(selfSystem))
	fmt.Fprintf(r.streams.Stdout, "%s %s\n", formatCPUTime(childUser), formatCPUTime(childSystem))
	return 0
}

// recordChildCPU adds a finished child's CPU to the shell's running total. A
// child that never started has no ProcessState and contributes nothing, which
// is also true of the process it models.
func (r Runtime) recordChildCPU(cmd *exec.Cmd) {
	if cmd.ProcessState == nil || r.childCPU == nil {
		return
	}
	r.childCPU.add(cmd.ProcessState.UserTime(), cmd.ProcessState.SystemTime())
}

func formatCPUTime(d time.Duration) string {
	minutes := int64(d / time.Minute)
	seconds := (d - time.Duration(minutes)*time.Minute).Seconds()
	return fmt.Sprintf("%dm%.3fs", minutes, seconds)
}

// childCPUTime accumulates what waited-for children have used. Shared across
// snapshots by pointer, because a pipeline stage's children are the shell's
// children too and `times` in the parent has to see them.
type childCPUTime struct {
	mutex  sync.Mutex
	user   time.Duration
	system time.Duration
}

func (c *childCPUTime) add(user, system time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.user += user
	c.system += system
}

func (c *childCPUTime) total() (time.Duration, time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.user, c.system
}
