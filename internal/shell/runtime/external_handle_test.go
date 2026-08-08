package runtime

import (
	"bytes"
	"io"
	"os"
	"sync"
	"testing"
)

// A child must be handed the real file when there is one, so it inherits the
// console instead of a pipe.
//
// This is not a performance detail. A console child asks the console what it
// is: `help.exe` writes through the console API when it has one, and writes
// code-page bytes when it has a pipe. Those bytes then reach Go's console
// writer, which decodes them as UTF-8, so CP936 output from a Chinese Windows
// arrives as replacement characters. Measured with a probe run under conhost:
// stdout was FILE_TYPE_PIPE through Nemosh and a console when run directly.
// The same wrapper also hides a terminal from anything that checks isatty --
// colours, progress bars, and pagers all turn themselves off.
func TestNativeWriter_unwrapsToTheUnderlyingFile(t *testing.T) {
	// Given
	file := os.Stdout
	mutex := &sync.Mutex{}

	for _, test := range []struct {
		name   string
		writer io.Writer
		want   io.Writer
	}{
		{
			name:   "a bare file is itself",
			writer: file,
			want:   file,
		},
		{
			name:   "one synchronized wrapper is unwrapped",
			writer: synchronizedWriter{mutex: mutex, writer: file},
			want:   file,
		},
		{
			name:   "nesting is unwrapped all the way down",
			writer: synchronizedWriter{mutex: mutex, writer: synchronizedWriter{mutex: mutex, writer: file}},
			want:   file,
		},
		{
			name:   "a writer with no file underneath is left alone",
			writer: synchronizedWriter{mutex: mutex, writer: &bytes.Buffer{}},
			want:   nil,
		},
		{
			name:   "a plain buffer has no file",
			writer: &bytes.Buffer{},
			want:   nil,
		},
		{
			name:   "nil is not a file",
			writer: nil,
			want:   nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := nativeWriter(test.writer)

			// Then
			if test.want == nil {
				if got != nil {
					t.Fatalf("nativeWriter() = %#v, want nil", got)
				}
				return
			}
			if got != test.want {
				t.Fatalf("nativeWriter() = %#v, want the underlying file", got)
			}
		})
	}
}

// Unwrapping must not change what a capturing caller sees: a pipeline stage
// collecting output into a buffer still has to go through the wrapper, because
// there is no file to hand over.
func TestNativeWriter_leavesCapturingWritersToTheWrapper(t *testing.T) {
	// Given
	var captured bytes.Buffer
	wrapped := synchronizedWriter{mutex: &sync.Mutex{}, writer: &captured}

	// When
	if native := nativeWriter(wrapped); native != nil {
		t.Fatalf("nativeWriter() = %#v, want nil so the wrapper stays in play", native)
	}
	if _, err := wrapped.Write([]byte("through")); err != nil {
		t.Fatal(err)
	}

	// Then
	if captured.String() != "through" {
		t.Fatalf("captured = %q, want %q", captured.String(), "through")
	}
}
