package applets

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// dump is od_bloaty.c's dump: blocks of width bytes after -j, no more than -N of them, the
// last padded with zeros to a whole number of each spec's datums, and then the length. The
// first FILE is opened and SKIP skipped before a bad -w is warned of, as busybox does.
func (r *odRun) dump(stdout, stderr io.Writer, input *dumpInputs) error {
	if _, err := input.Read(nil); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if err := r.skipBytes(input); err != nil {
		return err
	}
	if !input.opened {
		return nil
	}
	lcm := r.lcm()
	if !r.widthGiven {
		r.width = lcm * max(16/lcm, 1)
	} else if r.width == 0 || r.width%lcm != 0 {
		fmt.Fprintf(stderr, "od: warning: invalid width %d; using %d instead\n", r.width, lcm)
		r.width = lcm
	}
	var reader io.Reader = input
	if r.limited {
		reader = io.LimitReader(input, r.limit)
	}
	if r.stringsGiven {
		return r.dumpStrings(stdout, reader)
	}
	r.columns()
	offset := r.skip
	var current, previous []byte
	printed, starred := false, false
	for {
		var err error
		if current, err = readOdBlock(reader, current, r.width); err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		n := len(current)
		if n == 0 {
			break
		}
		if n == r.width && !r.verbose && printed && bytes.Equal(current, previous) {
			if !starred {
				if _, err := io.WriteString(stdout, "*\n"); err != nil {
					return err
				}
			}
			starred = true
		} else {
			block := append(current, make([]byte, (n+lcm-1)/lcm*lcm-n)...)
			if err := r.writeBlock(stdout, offset, block); err != nil {
				return err
			}
			printed, starred = true, false
		}
		offset += int64(n)
		current, previous = previous, current
		if n < r.width {
			break
		}
	}
	if r.addressing == odAddressNone {
		return nil
	}
	_, err := io.WriteString(stdout, r.address(offset)+"\n")
	return err
}

// readOdBlock is up to width bytes of input, into block's storage. It grows with what is
// read, so a -w of millions costs only what the input holds.
func readOdBlock(reader io.Reader, block []byte, width int) ([]byte, error) {
	block = block[:0]
	for len(block) < width {
		if len(block) == cap(block) {
			block = slices.Grow(block, min(width-len(block), max(len(block), 512)))
		}
		n, err := reader.Read(block[len(block):min(cap(block), width)])
		if block = block[:len(block)+n]; err != nil {
			return block, err
		}
	}
	return block, nil
}

// skipBytes is -j: SKIP bytes read and left, across FILEs, as busybox's skip does.
func (r *odRun) skipBytes(input io.Reader) error {
	if r.skip == 0 {
		return nil
	}
	skipped, err := io.CopyN(io.Discard, input, r.skip)
	if skipped < r.skip {
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		return errors.New("can't skip past end of combined input")
	}
	return nil
}

// writeBlock is od_bloaty.c's write_block for a block to be printed: a line per spec, the
// address before the first and blanks as wide before the rest, and -t's z after its line.
func (r *odRun) writeBlock(stdout io.Writer, offset int64, block []byte) error {
	var out strings.Builder
	for index, spec := range r.specs {
		if index == 0 {
			out.WriteString(r.address(offset))
		} else {
			// With no address, no blanks either: busybox reads -A n's width out of bounds.
			out.WriteString(strings.Repeat(" ", r.pad))
		}
		fields := r.width / spec.size
		spec.write(&out, block, fields, r.pads[index])
		if spec.hexl {
			blank := (r.width - len(block)) / spec.size
			out.WriteString(strings.Repeat(" ", blank*(spec.width+1)+r.pads[index]*blank/fields) + "  >")
			for _, b := range block {
				if b < 0x20 || b > 0x7e {
					b = '.'
				}
				out.WriteByte(b)
			}
			out.WriteByte('<')
		}
		out.WriteByte('\n')
	}
	_, err := io.WriteString(stdout, out.String())
	return err
}

// address is an offset as -A prints it, and with --traditional's LABEL the offset it names.
func (r *odRun) address(offset int64) string {
	number := func(offset int64) string {
		return fmt.Sprintf(map[byte]string{'o': "%0*o", 'u': "%0*d", 'x': "%0*x"}[r.radix], r.pad, uint64(offset))
	}
	switch r.addressing {
	case odAddressNone:
		return ""
	case odAddressParen:
		return "(" + number(offset) + ")"
	case odAddressLabel:
		return number(offset) + " (" + number(offset+r.pseudo) + ")"
	}
	return number(offset)
}

// dumpStrings is -S, od_bloaty.c's dump_strings: each run of MINSTR or more printable
// characters that ends in a NUL, after the address it begins at. One that -N's SIZE ends is
// printed too, as both busybox's and GNU's print it, though where it begins: theirs is the
// address before, which for a run from offset 0 is 1777777777777777777777.
func (r *odRun) dumpStrings(stdout io.Writer, input io.Reader) error {
	reader := bufio.NewReader(input)
	address, end := r.skip, r.skip+r.limit
	var run []byte
strings:
	for !r.limited || end-r.strings > address {
		start := address
		run = run[:0]
		for !r.limited || address < end {
			c, err := reader.ReadByte()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
			address++
			if c == 0 {
				break
			}
			if c < 0x20 || c > 0x7e {
				continue strings
			}
			run = append(run, c)
		}
		if int64(len(run)) < r.strings {
			continue
		}
		line := string(run) + "\n"
		if r.addressing != odAddressNone {
			line = r.address(start) + " " + line
		}
		if _, err := io.WriteString(stdout, line); err != nil {
			return err
		}
	}
	return nil
}
