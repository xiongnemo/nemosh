package proc

import (
	"errors"
	"os"
	"testing"
)

// A pid with nothing behind it is `cannot signal pid N: No such process`, on Windows too,
// where OpenProcess calls it an invalid parameter and the message was Windows' own "The
// parameter is incorrect."
func TestTerminate_aPidWithNoProcessIsNoSuchProcess(t *testing.T) {
	// When: zero, which only asks
	err := Terminate(99999999, 0)

	// Then
	if !errors.Is(err, ErrNoSuchProcess) || err.Error() != "cannot signal pid 99999999: No such process" {
		t.Fatalf("Terminate(99999999, 0) = %v, want cannot signal pid 99999999: No such process", err)
	}
}

// OpenProcess ignores a pid's low two bits, so `kill 13` opened 12 and ended it. A pid is checked
// against the process opened: one above this process's names no process, for a Windows pid is a
// multiple of four, though OpenProcess opens this one for it. Signal 0, which only asks.
func TestTerminate_aPidIsTheProcessItNames(t *testing.T) {
	pid := os.Getpid() + 1
	if err := Terminate(pid, 0); !errors.Is(err, ErrNoSuchProcess) {
		t.Fatalf("Terminate(%d, 0) = %v, want No such process", pid, err)
	}
	if _, err := AdjustPriority(pid, 1); err == nil {
		AdjustPriority(os.Getpid(), -1)
		t.Fatalf("AdjustPriority(%d) changed this process's priority", pid)
	}
}
