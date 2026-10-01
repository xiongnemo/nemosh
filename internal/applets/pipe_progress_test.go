package applets

import (
	"io"
	"strings"
	"testing"
)

// steppingReader hands out its chunks one read at a time and moves the clock pipe_progress reads
// by one second before each chunk it is told to.
type steppingReader struct {
	chunks []string
	later  map[int]bool
	clock  *int64
	read   int
}

func (r *steppingReader) Read(buffer []byte) (int, error) {
	if r.read == len(r.chunks) {
		return 0, io.EOF
	}
	if r.later[r.read] {
		*r.clock++
	}
	count := copy(buffer, r.chunks[r.read])
	r.read++
	return count, nil
}

// pipe_progress passes its input through untouched and puts a dot on stderr for each read that
// comes in a later second than the one before, and a newline at the end, as busybox's does.
func TestPipeProgress_marksEachSecondTheInputMoves(t *testing.T) {
	clock := int64(1000)
	saved := pipeProgressNow
	pipeProgressNow = func() int64 { return clock }
	t.Cleanup(func() { pipeProgressNow = saved })
	applet, _ := DefaultRegistry.Lookup("pipe_progress")
	var stdout, stderr strings.Builder

	// When: four reads, the second and fourth a second after the read before them
	input := &steppingReader{chunks: []string{"one\n", "two\n", "three\n", "four\n"}, later: map[int]bool{1: true, 3: true}, clock: &clock}
	err := applet.Run(t.Context(), []string{"-x", "ignored"}, input, &stdout, &stderr)

	// Then
	if err != nil || stdout.String() != "one\ntwo\nthree\nfour\n" || stderr.String() != "..\n" {
		t.Fatalf("err %v, stdout %q, stderr %q; want the input whole and ..\\n", err, stdout.String(), stderr.String())
	}
}

// With nothing to read there is still the newline, and the status is 0.
func TestPipeProgress_endsItsLineOnAnEmptyInput(t *testing.T) {
	stdout, stderr, status := runApplet(t, "pipe_progress", nil, "")
	if stdout != "" || stderr != "\n" || status != 0 {
		t.Fatalf("stdout %q, stderr %q, status %d", stdout, stderr, status)
	}
}
