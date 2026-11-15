package runtime

import (
	"os"
	"testing"
)

// The parent is read from the process's own basic information on Windows, rather than
// by the process snapshot os.Getppid walks; the two have to agree, since $PPID was the
// snapshot's answer before.
func TestParentProcessID_isTheParentTheSystemReports(t *testing.T) {
	// When
	got := parentProcessID()

	// Then
	if want := os.Getppid(); got != want {
		t.Fatalf("parentProcessID() = %d, want %d, as os.Getppid reports", got, want)
	}
}
