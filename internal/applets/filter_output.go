package applets

import (
	"bufio"
	"io"
	"os"
)

// A text filter's standard output, buffered and flushed whenever the filter is about to wait.
//
// sed, cut and head wrote each line as they made it, and tr each line into a pipe: a write per
// line, a system call each, which `seq 1000000 | sed s/1/x/` paid a million times. busybox's
// are stdio's and fill a whole buffer before a pipe sees anything, so `tail -f log | sed ...`
// shows nothing until 4K have gathered. These buffer what they write and flush it when the
// next read could wait: before each read their input's own buffer cannot answer, which is
// when a pipeline's writer has gone quiet and someone may be watching, and before anything is
// said on stderr, so the two keep their order. A read a buffer can answer flushes nothing, so
// a file read a chunk at a time is written a chunk at a time.

// filterOutput is the buffered standard output. It has no ReadFrom, which io.Copy would use:
// that reads into the buffer through the flushing input, whose flush empties the buffer
// under it, and `head -vc1` wrote its header's first byte where stdin's should be.
type filterOutput struct {
	buffer *bufio.Writer
	under  io.Writer
}

func newFilterOutput(stdout io.Writer) filterOutput {
	return filterOutput{buffer: bufio.NewWriterSize(stdout, 32*1024), under: stdout}
}

func (o filterOutput) Write(p []byte) (int, error)       { return o.buffer.Write(p) }
func (o filterOutput) WriteString(s string) (int, error) { return o.buffer.WriteString(s) }
func (o filterOutput) WriteByte(c byte) error            { return o.buffer.WriteByte(c) }
func (o filterOutput) WriteRune(r rune) (int, error)     { return o.buffer.WriteRune(r) }
func (o filterOutput) Flush() error                      { return o.buffer.Flush() }
func (o filterOutput) TerminalFile() *os.File            { return stdoutFile(o.under) }
func (o filterOutput) input(reader io.Reader) io.Reader  { return flushingReader{reader, o.buffer} }

// flushingReader is a filter's input, its output flushed before each read.
type flushingReader struct {
	reader io.Reader
	output *bufio.Writer
}

// Read flushes and then reads. A flush that fails is the filter's to report as a write error:
// the writer keeps the error, and the filter's next write, or its last flush, returns it.
func (r flushingReader) Read(p []byte) (int, error) {
	_ = r.output.Flush()
	return r.reader.Read(p)
}
