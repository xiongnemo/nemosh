package applets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
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
}

func (d *dumpInputs) Read(p []byte) (int, error) {
	for {
		if d.current == nil {
			if len(d.paths) == 0 {
				return 0, io.EOF
			}
			path := d.paths[0]
			d.paths = d.paths[1:]
			file, err := OpenProcessOperand(d.ctx, d.view, path, d.stdin)
			if err != nil {
				d.failed = true
				if failure := operandFailure(path, err); !reportOperand(d.ctx, failure) {
					return 0, failure
				}
				continue
			}
			d.current, d.opened = file, true
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

// write dumps input sixteen bytes to a line, and ends with the length. Unless verbose, a line
// the same as the one before it is not printed, and the first of a run of them is a `*`, as
// libbb's dump has it for hexdump and hd: every line was printed. It puts the `*` before a
// short last line too when it begins as the line before it, where od's write_block does not,
// and gives no length for no input.
func (r dumpRequest) write(stdout io.Writer, input *dumpInputs, verbose bool) error {
	current, previous := make([]byte, 16), make([]byte, 16)
	offset, printed, starred := 0, false, false
	for {
		n, err := io.ReadFull(input, current)
		if n == 0 {
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			break
		}
		repeated := !verbose && printed && bytes.Equal(current[:n], previous[:n])
		if repeated && n == len(current) {
			if !starred {
				if _, err := fmt.Fprintln(stdout, "*"); err != nil {
					return err
				}
			}
			starred = true
		} else {
			if repeated && !starred {
				if _, err := fmt.Fprintln(stdout, "*"); err != nil {
					return err
				}
			}
			if err := r.writeLine(stdout, offset, current[:n]); err != nil {
				return err
			}
			printed, starred = true, false
		}
		offset += n
		current, previous = previous, current
		if n < len(current) {
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				return err
			}
			break
		}
	}
	// The final line is the length, which is how a reader knows where the dump
	// stopped without counting the rows. With -A n there is no address to give, and so
	// no line, as in busybox: it was an empty one, which every `... | od -A n -c` then
	// carried into whatever read it. Nor is there one when no FILE could be opened, or when
	// there was nothing to dump.
	if r.radix == 'n' || !input.opened || offset == 0 {
		return nil
	}
	_, err := fmt.Fprintln(stdout, strings.TrimSpace(r.address(offset)))
	return err
}

// writeLine is one line of the dump, a line for each format, the address on the first.
//
// Not trimmed: hexdump's word form pads its line out to eight slots and od's does not, so the
// padding is part of the body rather than something to tidy away here. Measured against both.
func (r dumpRequest) writeLine(stdout io.Writer, offset int, chunk []byte) error {
	address := r.address(offset)
	for index, body := range r.bodies(chunk) {
		if index > 0 {
			address = strings.Repeat(" ", len(address))
		}
		if _, err := fmt.Fprintln(stdout, address+body); err != nil {
			return err
		}
	}
	return nil
}
