package applets

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

// dumper is libbb's dumper for hexdump and hd: the -e formats, -v, -n's LENGTH and -s's
// OFFSET, and the unit holding the %_A that is printed after the last block.
type dumper struct {
	formats []*dumpFormat
	ending  *dumpUnit
	verbose bool
	length  int64
	skip    int64
}

// prepare is libbb's bb_dump_dump before the display: each format's conversions, the block as
// long as the longest format, and the last unit of each repeated to fill it.
func (d *dumper) prepare() (int, error) {
	block := 0
	for _, format := range d.formats {
		for _, unit := range format.units {
			if err := unit.rewrite(); err != nil {
				return 0, err
			}
			if unit.ending {
				d.ending = unit
			}
			format.bytes += unit.bytes * unit.reps
		}
		block = max(block, format.bytes)
	}
	for _, format := range d.formats {
		format.fill(block)
	}
	return block, nil
}

// dumpBlocks is libbb's get over a stream: its blocks in turn, a run of them the same as the
// one before passed over and marked by a `*`, the last padded with zeros.
type dumpBlocks struct {
	input           io.Reader
	out             *bufio.Writer
	current, saved  []byte
	started         bool
	verbose, repeat bool
	// waiting is whether a block has been printed that the next may repeat.
	waiting bool
	// address is the offset of the current block, savedAddress that of the last one printed
	// or passed over, and end where the input ends, or 0 while it has not.
	address, savedAddress, end int64
}

func (b *dumpBlocks) next() ([]byte, error) {
	if b.started {
		b.current, b.saved = b.saved, b.current
		b.savedAddress += int64(len(b.current))
		b.address = b.savedAddress
	}
	b.started = true
	for {
		n, err := io.ReadFull(b.input, b.current)
		if n < len(b.current) {
			if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				return nil, err
			}
			if n == 0 {
				return nil, nil
			}
			// A short last block the same as the one before as far as it goes has a `*` before
			// it, unless one already stands for a run.
			if !b.verbose && b.waiting && !b.repeat && bytes.Equal(b.current[:n], b.saved[:n]) {
				b.out.WriteString("*\n")
			}
			clear(b.current[n:])
			b.end = b.address + int64(n)
			return b.current, nil
		}
		if b.verbose || !b.waiting || !bytes.Equal(b.current, b.saved) {
			b.waiting, b.repeat = true, false
			return b.current, nil
		}
		if !b.repeat {
			b.out.WriteString("*\n")
		}
		b.repeat = true
		b.savedAddress += int64(len(b.current))
		b.address = b.savedAddress
	}
}

// run is libbb's display: every format over each block, and then the %_A unit's address.
func (d *dumper) run(stdout io.Writer, inputs *dumpInputs) error {
	block, err := d.prepare()
	if err != nil {
		return err
	}
	// With -n 0 nothing is read, and not even the FILEs are opened.
	if d.length == 0 {
		return nil
	}
	out := bufio.NewWriterSize(stdout, 32<<10)
	inputs.flush = out.Flush
	skipped, err := io.CopyN(io.Discard, inputs, d.skip)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	var input io.Reader = inputs
	if d.length > 0 {
		input = io.LimitReader(inputs, d.length)
	}
	// A block of no bytes reads nothing, though each FILE is opened, and named if it cannot be.
	if block == 0 {
		if err := inputs.openEach(); err != nil {
			return err
		}
	}
	blocks := &dumpBlocks{input: input, out: out, current: make([]byte, block), saved: make([]byte, block),
		verbose: d.verbose, address: skipped, savedAddress: skipped}
	for block > 0 {
		data, err := blocks.next()
		if err != nil {
			out.Flush()
			return err
		}
		if data == nil {
			break
		}
		for _, format := range d.formats {
			if err := d.display(out, format, data, blocks.address, blocks.end); err != nil {
				return err
			}
		}
	}
	d.finish(out, blocks.end, blocks.address)
	return out.Flush()
}

// display is one format over a block. A conversion past the end of the input prints blanks
// as wide, and a unit holding %_A ends the format's line.
func (d *dumper) display(out *bufio.Writer, format *dumpFormat, data []byte, address, end int64) error {
	at := 0
	for _, unit := range format.units {
		if unit.ending {
			return nil
		}
		for count := unit.reps; count > 0; count-- {
			for index := range unit.prints {
				print := &unit.prints[index]
				if end != 0 && address >= end && print.kind != dumpText && print.kind != dumpBlank {
					print.kind = dumpBlank
				}
				if _, err := out.WriteString(print.render(data, at, address, count == 1)); err != nil {
					return err
				}
				address += int64(print.bytes)
				at += print.bytes
			}
		}
	}
	return nil
}

// finish prints the %_A unit's addresses as where the input ended, or nothing when there was
// no input at all.
func (d *dumper) finish(out *bufio.Writer, end, address int64) {
	if d.ending == nil {
		return
	}
	if end == 0 {
		if address == 0 {
			return
		}
		end = address
	}
	for index := range d.ending.prints {
		switch print := &d.ending.prints[index]; print.kind {
		case dumpAddress, dumpText:
			out.WriteString(print.render(nil, 0, end, false))
		}
	}
}
