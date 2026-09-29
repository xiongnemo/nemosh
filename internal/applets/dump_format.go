package applets

import (
	"fmt"
	"strconv"
	"strings"
)

// The format hexdump prints by, libbb's dump.c after util-linux: each -e is a dumpFormat, a
// run of units printed in turn over every block of input, and a unit is `[COUNT][/BYTES]
// "TEXT"`, TEXT printed COUNT times, each over BYTES of the block. TEXT is printf's, one
// conversion for each datum.

// dumpFormat is libbb's FS: one -e's units.
type dumpFormat struct {
	units []*dumpUnit
	// bytes is what one pass over the units reads, bb_dump_size's.
	bytes int
}

// dumpUnit is libbb's FU.
type dumpUnit struct {
	reps int
	// setReps is whether COUNT was given, so that the last unit is not repeated to fill the
	// block.
	setReps bool
	// ending is a unit holding %_A: it is printed once, after the last block, and the units
	// after it in its -e never are.
	ending bool
	bytes  int
	text   string
	prints []dumpPrint
}

// parseDumpFormat is libbb's bb_dump_add: the units of one -e.
func parseDumpFormat(format string) (*dumpFormat, error) {
	bad := fmt.Errorf("bad format {%s}", format)
	parsed := &dumpFormat{}
	at := 0
	blanks := func() {
		for at < len(format) && isCSpace(format[at]) {
			at++
		}
	}
	number := func() int {
		start := at
		for at < len(format) && isASCIIDigit(format[at]) {
			at++
		}
		value, err := strconv.Atoi(format[start:at])
		if err != nil {
			value = 1<<31 - 1
		}
		return value
	}
	for blanks(); at < len(format); blanks() {
		unit := &dumpUnit{reps: 1}
		if isASCIIDigit(format[at]) {
			unit.reps, unit.setReps = number(), true
			if at == len(format) || !isCSpace(format[at]) && format[at] != '/' {
				return nil, bad
			}
			// The blank or the slash after COUNT is passed over, and then any blanks.
			at++
			blanks()
		}
		if at < len(format) && format[at] == '/' {
			at++
			blanks()
		}
		if at < len(format) && isASCIIDigit(format[at]) {
			if unit.bytes = number(); at == len(format) || !isCSpace(format[at]) {
				return nil, bad
			}
			at++
			blanks()
		}
		if at == len(format) || format[at] != '"' {
			return nil, bad
		}
		end := strings.IndexByte(format[at+1:], '"')
		if end < 0 {
			return nil, bad
		}
		unit.text = dumpEscapes(format[at+1 : at+1+end])
		at += end + 2
		parsed.units = append(parsed.units, unit)
	}
	return parsed, nil
}

// dumpEscapes is busybox's strcpy_and_process_escape_sequences: \a \b \e \f \n \r \t \v and
// \\, an octal number of up to three digits, \x and up to two hex digits; any other keeps its
// backslash. A NUL ends the text, as it ends the C string busybox prints it from.
func dumpEscapes(text string) string {
	var out strings.Builder
	for at := 0; at < len(text); at++ {
		c := text[at]
		if c == '\\' {
			var used int
			c, used = dumpEscape(text[at+1:])
			at += used
		}
		if c == 0 {
			break
		}
		out.WriteByte(c)
	}
	return out.String()
}

// dumpEscape is busybox's bb_process_escape_sequence: the byte a backslash and rest stand
// for, and how much of rest it took.
func dumpEscape(rest string) (byte, int) {
	base, start, digits := 8, 0, 3
	if rest != "" && rest[0] == 'x' {
		base, start, digits = 16, 1, 2
	}
	value, end := 0, start
	for end < len(rest) && end-start < digits {
		digit, err := strconv.ParseUint(rest[end:end+1], base, 8)
		if err != nil || value*base+int(digit) > 0xff {
			break
		}
		value, end = value*base+int(digit), end+1
	}
	switch {
	case end > start:
		return byte(value), end
	case base == 16:
		return '\\', 0
	case rest != "":
		if index := strings.IndexByte("abefnrtv\\", rest[0]); index >= 0 {
			return "\a\b\x1b\f\n\r\t\v\\"[index], 1
		}
	}
	return '\\', 0
}
