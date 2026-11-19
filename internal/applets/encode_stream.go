package applets

import (
	"bufio"
	"errors"
	"io"
)

// base64 and base32 stream, as busybox's do: `base64 -d` of 26 MB peaked at 155 MiB of commit
// where busybox's peaked at 1.1, as the whole input was read, cleaned into a copy and decoded
// into a third, and 3 MB took some 40 MB of allocations each way. What is written is the same:
// the whole-input encoding in lines of the width, and for a decode the same bytes, or `invalid
// input` -- after the part that decoded, now, as GNU's, which this was measured against, says it.

// lineWrapper writes what it is handed in lines of width, a newline after each and after a last
// one that is short. Width zero is no newline at all, as GNU's -w0 is: `base64 -w0 | wc -l` is 0,
// and anyone piping the output onward is relying on that.
type lineWrapper struct {
	out           *bufio.Writer
	width, column int
}

func (w *lineWrapper) Write(text []byte) (int, error) {
	if w.width == 0 {
		return w.out.Write(text)
	}
	written := len(text)
	for len(text) > 0 {
		length := min(w.width-w.column, len(text))
		if _, err := w.out.Write(text[:length]); err != nil {
			return 0, err
		}
		w.column, text = w.column+length, text[length:]
		if w.column == w.width {
			if err := w.out.WriteByte('\n'); err != nil {
				return 0, err
			}
			w.column = 0
		}
	}
	return written, nil
}

// finish ends a line left short, and writes out what is held.
func (w *lineWrapper) finish() error {
	if w.width > 0 && w.column > 0 {
		if err := w.out.WriteByte('\n'); err != nil {
			return err
		}
	}
	return w.out.Flush()
}

// encodeStream copies reader through the encoder newEncoder makes, in lines of width.
func encodeStream(reader io.Reader, stdout io.Writer, width int, newEncoder func(io.Writer) io.WriteCloser) error {
	wrapped := &lineWrapper{out: bufio.NewWriterSize(stdout, 32<<10), width: width}
	encoder := newEncoder(wrapped)
	// A buffer at a time, not through the reader's WriteTo: a strings.Reader's writes all it has
	// in one, as a copy of it, which for 3 MB was a 3 MB slice.
	if _, err := io.CopyBuffer(encoder, struct{ io.Reader }{reader}, make([]byte, 32<<10)); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	return wrapped.finish()
}

// encodedText is the text a decoder reads: its line endings dropped, which is not politeness
// but necessity, as the encoder's own output is wrapped; under -i any byte keep refuses as well;
// and, where padTo is set, padding to a whole block at the end. A failure to read is kept, to be
// reported as it is rather than as invalid input.
type encodedText struct {
	reader  io.Reader
	keep    func(byte) bool
	padTo   int
	passed  int
	padding int
	ended   bool
	err     error
}

func (t *encodedText) Read(buffer []byte) (int, error) {
	for {
		if t.ended {
			if t.padding == 0 {
				return 0, io.EOF
			}
			length := min(len(buffer), t.padding)
			for index := range length {
				buffer[index] = '='
			}
			t.padding -= length
			return length, nil
		}
		read, err := t.reader.Read(buffer)
		kept := 0
		for _, character := range buffer[:read] {
			if character == '\n' || character == '\r' || t.keep != nil && !t.keep(character) {
				continue
			}
			buffer[kept] = character
			kept++
		}
		t.passed += kept
		switch {
		case errors.Is(err, io.EOF):
			t.ended = true
			if t.padTo > 0 && t.passed%t.padTo != 0 {
				t.padding = t.padTo - t.passed%t.padTo
			}
		case err != nil:
			t.err = err
			return kept, err
		}
		if kept > 0 {
			return kept, nil
		}
	}
}

// decodeStream writes what decoder reads out of text, and says `invalid input` where the
// decoder fails, after writing what it decoded before.
func decodeStream(stdout io.Writer, text *encodedText, decoder io.Reader) error {
	out := bufio.NewWriterSize(stdout, 32<<10)
	buffer := make([]byte, 32<<10)
	for {
		read, err := decoder.Read(buffer)
		if read > 0 {
			if _, writeErr := out.Write(buffer[:read]); writeErr != nil {
				return writeErr
			}
		}
		if errors.Is(err, io.EOF) {
			return out.Flush()
		}
		if err != nil {
			if flushErr := out.Flush(); flushErr != nil {
				return flushErr
			}
			if text.err != nil {
				return text.err
			}
			return errors.New("invalid input")
		}
	}
}
