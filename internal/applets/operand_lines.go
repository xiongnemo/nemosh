package applets

import (
	"bufio"
	"context"
	"io"
)

// operandLines is one operand's lines, a named file or stdin for `-`, read as they are wanted:
// split as bufio.Scanner splits them, a CR before a newline dropped, at most maxTextLine long.
// comm and paste read every operand whole, then split it, and wrote nothing until the inputs
// ended: `paste - -` of 19 MB peaked at 95 MiB, comm of two 4 MB files at 52, busybox's at 1.1.
type operandLines struct {
	scanner *bufio.Scanner
	closer  io.Closer
	ended   bool
}

func openOperandLines(ctx context.Context, view ProcessView, path string, stdin io.Reader, out *bufio.Writer) (*operandLines, error) {
	reader, closer := stdin, io.Closer(nil)
	if path != "-" {
		file, err := OpenProcessInput(ctx, view, path)
		if err != nil {
			// cmp, comm, join and paste open theirs as fopen_or_warn does, which names one it
			// cannot `FILE: No such file or directory`.
			return nil, operandFailure(path, err)
		}
		reader, closer = file, file
	}
	scanner := bufio.NewScanner(flushBeforeRead{reader: reader, out: out})
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	return &operandLines{scanner: scanner, closer: closer}, nil
}

// next is the next line, and false once there are no more.
func (o *operandLines) next() (string, bool) {
	if !o.ended && o.scanner.Scan() {
		return o.scanner.Text(), true
	}
	o.ended = true
	return "", false
}

func (o *operandLines) close() error {
	if o.closer == nil {
		return nil
	}
	return o.closer.Close()
}

// flushBeforeRead writes out what is held before a read, which may wait: so a line is out as
// soon as it is made, `tail -f log | paste - -` shows each row as it comes, and a file is written
// a buffer at a time rather than a line.
type flushBeforeRead struct {
	reader io.Reader
	out    *bufio.Writer
}

func (f flushBeforeRead) Read(buffer []byte) (int, error) {
	if err := f.out.Flush(); err != nil {
		return 0, err
	}
	return f.reader.Read(buffer)
}
