package applets

import (
	"bufio"
	"context"
	"errors"
	"io"
)

// dumpInputs is every FILE as one stream, as busybox's od and hexdump read them: the offsets
// run on from one FILE into the next, each opened when the one before is done, and one that
// cannot be opened is named, `od: FILE: No such file or directory`, and passed over. Each
// FILE was dumped by itself, its offsets from 0 again.
type dumpInputs struct {
	ctx     context.Context
	view    ProcessView
	stdin   io.Reader
	paths   []string
	current io.ReadCloser
	// opened is whether any FILE was, and failed whether one was not.
	opened, failed bool
	// flush writes out what the dump holds before each FILE is opened, so that what came before
	// one that cannot be is printed before it is named, as a terminal's line buffering has it.
	flush func() error
	// exact is whether stdin is read no further than the dump goes, as od -N and hexdump -n
	// read it, so that `{ od -N 4; cat; } < f` leaves cat the rest. A FILE is read a buffer at
	// a time, where each dump's line had been a read of its own.
	exact bool
}

// bufferedFile is a FILE read a buffer at a time.
type bufferedFile struct {
	*bufio.Reader
	file io.Closer
}

func (b bufferedFile) Close() error { return b.file.Close() }

// open is the next FILE, and whether there was one; one that cannot be opened is named, and
// the error is returned when the shell would stop there.
func (d *dumpInputs) open() (io.ReadCloser, bool, error) {
	for len(d.paths) > 0 {
		if d.flush != nil {
			if err := d.flush(); err != nil {
				return nil, false, err
			}
		}
		path := d.paths[0]
		d.paths = d.paths[1:]
		file, err := OpenProcessOperand(d.ctx, d.view, path, d.stdin)
		if err == nil {
			d.opened = true
			if path != "-" || !d.exact {
				file = bufferedFile{bufio.NewReaderSize(file, 64<<10), file}
			}
			return file, true, nil
		}
		d.failed = true
		if failure := operandFailure(path, err); !reportOperand(d.ctx, failure) {
			return nil, false, failure
		}
	}
	return nil, false, nil
}

func (d *dumpInputs) Read(p []byte) (int, error) {
	for {
		if d.current == nil {
			file, ok, err := d.open()
			if err != nil {
				return 0, err
			}
			if !ok {
				return 0, io.EOF
			}
			d.current = file
		}
		n, err := d.current.Read(p)
		if !errors.Is(err, io.EOF) {
			return n, err
		}
		closeErr := d.current.Close()
		d.current = nil
		if n > 0 || closeErr != nil {
			return n, closeErr
		}
	}
}

// Close closes the FILE being read, which od's -N and -S can stop in, and an error can leave.
func (d *dumpInputs) Close() error {
	if d.current == nil {
		return nil
	}
	err := d.current.Close()
	d.current = nil
	return err
}

// openEach opens and closes each FILE left without reading it, naming the ones it cannot
// open, as hexdump does with a format that reads no bytes.
func (d *dumpInputs) openEach() error {
	for d.Close(); ; {
		file, ok, err := d.open()
		if !ok || err != nil {
			return err
		}
		file.Close()
	}
}
