package proc

import (
	"errors"
	"fmt"
	"math"

	"golang.org/x/sys/windows"
)

// Watcher is a process watched for its end, which is what tail --pid asks at every look: GNU's
// kill(pid, 0) in tail_forever. A handle is held rather than the pid asked about each time, so a
// pid that a new process takes once the watched one ends is not mistaken for it.
type Watcher struct {
	handle windows.Handle
}

// Watch opens pid to learn when it ends. One that names no process has ended already, which is
// the answer rather than an error -- and that includes 13, which OpenProcess would have opened 12
// for (see openProcess), so `tail --pid=13` followed whatever ran as 12. One this session may not
// open is refused, where GNU's tail takes EPERM as alive and would follow until interrupted.
func Watch(pid int) (*Watcher, error) {
	// Widened, for on windows/386 no int is above MaxUint32 and the comparison would not compile.
	if pid <= 0 || int64(pid) > math.MaxUint32 {
		return nil, fmt.Errorf("invalid PID: '%d'", pid)
	}
	handle, err := openProcess(windows.SYNCHRONIZE, pid)
	switch {
	case errors.Is(err, windows.ERROR_INVALID_PARAMETER):
		return &Watcher{}, nil
	case err != nil:
		return nil, fmt.Errorf("cannot watch process %d: %v", pid, killCause(err))
	}
	return &Watcher{handle: handle}, nil
}

// Ended is whether the process has ended, asked without waiting.
func (w *Watcher) Ended() bool {
	if w.handle == 0 {
		return true
	}
	event, err := windows.WaitForSingleObject(w.handle, 0)
	return err != nil || event == windows.WAIT_OBJECT_0
}

// Close lets the process go.
func (w *Watcher) Close() {
	if w.handle != 0 {
		windows.CloseHandle(w.handle)
		w.handle = 0
	}
}
