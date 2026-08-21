//go:build windows

package proc

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// Changing a process's priority, which is F7 and F8 in htop and `renice` everywhere else.
//
// Windows has priority *classes* where POSIX has a nice value from -20 to 19, so there is no
// arithmetic to do -- only a ladder to step up and down. That is a coarser instrument than nice and
// it is the whole of what the platform offers: five classes a person should use, and a sixth,
// REALTIME, which is deliberately unreachable here. A realtime process outranks the kernel's own
// input and disk threads, so raising one by keypress is how a machine stops responding to the
// keyboard that did it. htop cannot do this either without root; the difference is that Windows
// gates it behind SeIncreaseBasePriorityPrivilege rather than behind a signed integer.
//
// Only processes this session may open. Everything else is refused with the reason, which is the
// same posture the rest of internal/proc takes: opening handles is what costs privilege, and a
// monitor that needs none must say so rather than fail obscurely.

// priorityLadder is the classes worth stepping between, lowest first.
var priorityLadder = []uint32{
	windows.IDLE_PRIORITY_CLASS,
	windows.BELOW_NORMAL_PRIORITY_CLASS,
	windows.NORMAL_PRIORITY_CLASS,
	windows.ABOVE_NORMAL_PRIORITY_CLASS,
	windows.HIGH_PRIORITY_CLASS,
}

// priorityNames are the same classes as a person would say them, matching the PRI column.
var priorityNames = map[uint32]string{
	windows.IDLE_PRIORITY_CLASS:         "idle",
	windows.BELOW_NORMAL_PRIORITY_CLASS: "below",
	windows.NORMAL_PRIORITY_CLASS:       "norm",
	windows.ABOVE_NORMAL_PRIORITY_CLASS: "above",
	windows.HIGH_PRIORITY_CLASS:         "high",
	windows.REALTIME_PRIORITY_CLASS:     "real",
}

// AdjustPriority moves a process one step along the ladder and reports where it landed.
//
// One step rather than a target, because that is what a keypress means and because it keeps the
// caller from having to know the ladder. A process already at either end is left alone and says so:
// silently doing nothing is indistinguishable from a refused handle.
func AdjustPriority(pid, step int) (string, error) {
	if pid <= 0 {
		return "", fmt.Errorf("%d: not a process this can change", pid)
	}
	const access = windows.PROCESS_SET_INFORMATION | windows.PROCESS_QUERY_LIMITED_INFORMATION
	handle, err := windows.OpenProcess(access, false, uint32(pid))
	if err != nil {
		// The common case on this platform, and worth naming rather than passing up a
		// Win32 code: a process owned by SYSTEM or by another user cannot be opened for
		// this at all, elevated or not.
		return "", fmt.Errorf("%d: cannot change the priority of a process this session does not own", pid)
	}
	defer windows.CloseHandle(handle)

	current, err := windows.GetPriorityClass(handle)
	if err != nil {
		return "", fmt.Errorf("%d: %w", pid, err)
	}
	index := indexOfPriority(current)
	if index < 0 {
		// REALTIME, or a class this ladder does not carry. Stepping down from realtime is
		// safe and useful; stepping up is not, and there is nothing above it anyway.
		if current == windows.REALTIME_PRIORITY_CLASS && step < 0 {
			index = len(priorityLadder)
		} else {
			return "", fmt.Errorf("%d: priority %s is outside the range this changes", pid,
				priorityName(current))
		}
	}
	target := index + step
	if target < 0 || target >= len(priorityLadder) {
		return "", fmt.Errorf("%d: already at %s, which is as %s as this goes", pid,
			priorityName(current), directionWord(step))
	}
	if err := windows.SetPriorityClass(handle, priorityLadder[target]); err != nil {
		return "", fmt.Errorf("%d: %w", pid, err)
	}
	return priorityName(priorityLadder[target]), nil
}

func indexOfPriority(class uint32) int {
	for index, candidate := range priorityLadder {
		if candidate == class {
			return index
		}
	}
	return -1
}

func priorityName(class uint32) string {
	if name, ok := priorityNames[class]; ok {
		return name
	}
	return fmt.Sprintf("class %d", class)
}

func directionWord(step int) string {
	if step < 0 {
		return "low"
	}
	return "high"
}
