package applets

import (
	"errors"
	"io"
)

// iconvConverter is a conversion from one character set to another, win_iconv's rec_iconv_t,
// and convert its iconv(): a character at a time, through UTF-16, each mapped through the
// compatibility tables of either side.
type iconvConverter struct {
	from, to iconvCharset
	wide     []uint16 // a run's characters as UTF-16; see iconvRun
}

// convert converts what it can of in onto out: how much of in it took, and why it stopped
// short. Ignoring, -c, a byte that is no character and a character the target has not got are
// passed over; a character in stops inside is left for the next call, or at the end, last,
// passed over too. win_iconv passes it over wherever the buffer ends, which drops a character
// fread happened to split; this does not.
func (c *iconvConverter) convert(in []byte, out []byte, last bool) (int, []byte, error) {
	ignoring := c.to.flags&iconvIgnore != 0
	var wide [iconvCharMax]uint16
	var scratch [2 * iconvCharMax]byte
	used, slow := 0, 0
	for used < len(in) {
		if used >= slow {
			window := in[used:min(len(in), used+iconvWindow)]
			if took, more, ok := c.iconvRun(window, out); ok {
				used, out = used+took, more
				continue
			}
			// What the window holds goes a character at a time, and the next window is tried again.
			slow = used + len(window)
		}
		fromMode, toMode := c.from.mode, c.to.mode
		took, made, err := c.from.toWide(&c.from, in[used:], wide[:])
		if err != nil {
			c.from.mode = fromMode
			if errors.Is(err, errIconvIncomplete) && !last {
				return used, out, nil
			}
			if !ignoring {
				return used, out, err
			}
			took, made = 1, 0
		}
		if made == 0 {
			used += took
			continue
		}
		made = iconvCompatMap(c.from.compat, true, wide[:], made)
		made = iconvCompatMap(c.to.compat, false, wide[:], made)
		written, err := c.to.fromWide(&c.to, wide[:made], scratch[:])
		if err != nil {
			if !ignoring {
				c.from.mode, c.to.mode = fromMode, toMode
				return used, out, err
			}
			c.to.mode, written = toMode, 0
		}
		out = append(out, scratch[:written]...)
		used += took
	}
	return used, out, nil
}

// iconvWindow is how much input a run converts at most, and so how much goes a character at a
// time when one cannot.
const iconvWindow = 4096

// iconvCompatMap maps the character in wide through a compatibility table, inward on the way
// in, and says how many units it is now.
func iconvCompatMap(table []iconvCompat, inward bool, wide []uint16, made int) int {
	if table == nil {
		return made
	}
	point := utf16ToCodePoint(wide[:made])
	for _, entry := range table {
		if inward && entry.inward && entry.out == point {
			return codePointToUTF16(entry.in, wide)
		}
		if !inward && entry.in == point {
			return codePointToUTF16(entry.out, wide)
		}
	}
	return made
}

// finish is what ends the conversion, an escape back to ASCII for ISO-2022-JP, and the state of
// both sides made new for the next input, as win_iconv's iconv(cd, NULL, ...) does.
func (c *iconvConverter) finish(out []byte) []byte {
	if c.to.flush != nil {
		var scratch [iconvCharMax]byte
		written, _ := c.to.flush(&c.to, scratch[:])
		out = append(out, scratch[:written]...)
	}
	c.from.mode, c.to.mode = 0, 0
	return out
}

// convertStream is busybox's process_file: input read a block at a time, each converted and
// written as it is, so a conversion that fails has written what came before the failure, as
// busybox's does, and says why.
func (c *iconvConverter) convertStream(output io.Writer, input io.Reader) error {
	buffer := make([]byte, 32<<10)
	var converted []byte
	carried := 0
	for {
		read, readErr := input.Read(buffer[carried:])
		last := readErr != nil
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		used, out, err := c.convert(buffer[:carried+read], converted[:0], last)
		converted = out
		if _, writeErr := output.Write(converted); writeErr != nil {
			return writeErr
		}
		if err != nil {
			return iconvConversionError{err}
		}
		carried = copy(buffer, buffer[used:carried+read])
		if last {
			_, writeErr := output.Write(c.finish(converted[:0]))
			return writeErr
		}
	}
}

// iconvConversionError is busybox's bb_perror_msg_and_die("conversion error"): the words, and
// strerror's for why.
type iconvConversionError struct{ cause error }

func (e iconvConversionError) Error() string { return "conversion error: " + e.cause.Error() }
