package proc

import (
	"errors"
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
