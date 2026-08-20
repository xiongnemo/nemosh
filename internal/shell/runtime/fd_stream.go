package runtime

import (
	"context"
	"io"
	"os"
)

type descriptorReader struct {
	table *fdTable
	fd    int
}

func (r descriptorReader) Read(buffer []byte) (int, error) {
	reader, err := r.table.reader(r.fd)
	if err != nil {
		return 0, err
	}
	return reader.Read(buffer)
}

func (r descriptorReader) ReadContext(ctx context.Context, buffer []byte) (int, error) {
	reader, err := r.table.reader(r.fd)
	if err != nil {
		return 0, err
	}
	return readWithContext(ctx, reader, buffer)
}

func (r descriptorReader) LeaseStdinFile(ctx context.Context) (*os.File, func(), bool) {
	reader, err := r.table.reader(r.fd)
	if err != nil {
		return nil, func() {}, false
	}
	if file, ok := reader.(*os.File); ok {
		return file, func() {}, true
	}
	leaser, ok := reader.(stdinFileLeaser)
	if !ok {
		return nil, func() {}, false
	}
	return leaser.LeaseStdinFile(ctx)
}

type descriptorWriter struct {
	table *fdTable
	fd    int
}

func (w descriptorWriter) Write(buffer []byte) (int, error) {
	writer, err := w.table.writer(w.fd)
	if err != nil {
		return 0, err
	}
	return writer.Write(buffer)
}

// TerminalFile is the file this descriptor ends at, or nil when it is not a file.
//
// It exists so an applet can ask whether its output is a terminal. Everything an applet writes
// to goes through a descriptorWriter rather than through os.Stdout, so the obvious test --
// `stdout.(*os.File)` -- is always false inside the shell. Two things were quietly wrong because
// of that: `ls` never laid out columns even on a terminal, and `ls --color=auto` never coloured
// anything. Both looked like the feature was missing rather than like the question was being
// asked of the wrong object.
//
// nil for a pipe or a redirection, which is the answer that matters: a redirected `ls` should
// behave as it does into a pipe.
func (w descriptorWriter) TerminalFile() *os.File {
	entry, err := w.table.lookup(w.fd)
	if err != nil || entry.description == nil {
		return nil
	}
	return terminalFileOf(entry.description.writer)
}

// terminalFileOf walks a chain of writers to the file at the end of it.
//
// A chain, not one hop: fillStreams wraps stdout in a synchronizedWriter so that stdout and
// stderr cannot interleave, so even the top-level shell's stdout is two objects away from the
// file. Checking only the first was why this returned nil for an interactive shell, which a test
// caught immediately and reading the code had not.
func terminalFileOf(writer io.Writer) *os.File {
	switch target := writer.(type) {
	case *os.File:
		return target
	case interface{ TerminalFile() *os.File }:
		return target.TerminalFile()
	}
	return nil
}
