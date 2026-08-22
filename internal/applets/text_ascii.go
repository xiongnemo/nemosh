package applets

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// The `ascii` table: eight columns of sixteen rows, read *down* rather than
// across, so the first column is 0-15 and the last is 112-127.
//
// busybox's column spacing is hand-tuned and irregular -- the gaps between
// columns are 11, 11, 9, 9, 9, 10, 10 characters -- so it cannot be derived from
// a single format string. asciiNumberEdge below is that layout measured from
// `busybox ascii` on 2026-08-22, and the differential test is what keeps it
// honest. Reading the table across instead of down is the obvious implementation
// and produces a completely different, wrong table.

// asciiNumberEdge is the column each cell's decimal number ends at.
var asciiNumberEdge = [8]int{3, 14, 25, 34, 43, 52, 62, 72}

func newAsciiApplet() Applet {
	return simpleApplet{name: "ascii", runContext: func(_ context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		if _, operands, err := parseAppletOptions(args, "", ""); err != nil {
			return err
		} else if len(operands) > 0 {
			return fmt.Errorf("extra operand '%s'", operands[0])
		}
		for _, line := range asciiTable() {
			if _, err := fmt.Fprintln(stdout, line); err != nil {
				return err
			}
		}
		return nil
	}}
}

func asciiTable() []string {
	lines := make([]string, 0, 17)
	lines = append(lines, asciiRow(func(int) (string, string, string) { return "Dec", "Hex", "" }))
	for row := range 16 {
		lines = append(lines, asciiRow(func(column int) (string, string, string) {
			code := byte(column*16 + row)
			return fmt.Sprintf("%d", code), fmt.Sprintf("%02x", code), asciiName(code)
		}))
	}
	return lines
}

// asciiRow places eight cells at their measured offsets.
//
// The number is right-aligned to end at its column's edge, which is what makes a
// three-digit code line up with a one-digit one.
func asciiRow(cell func(column int) (number, hex, name string)) string {
	var line strings.Builder
	for column := range 8 {
		number, hex, name := cell(column)
		if pad := asciiNumberEdge[column] - len(number) - line.Len(); pad > 0 {
			line.WriteString(strings.Repeat(" ", pad))
		}
		line.WriteString(number + " " + hex + " " + name)
	}
	// Trailing blanks are not written: the space character's own row would
	// otherwise end in them, and no reference emits them.
	return strings.TrimRight(line.String(), " ")
}

// asciiName is the control-character mnemonic, or the character itself. The space
// at 0x20 has no name and prints as nothing, which is why the header's own cells
// are blank there too.
func asciiName(code byte) string {
	names := [...]string{
		"NUL", "SOH", "STX", "ETX", "EOT", "ENQ", "ACK", "BEL",
		// 0x0a is NL here, not LF: that is the spelling busybox's table uses, and
		// the whole table is compared against it byte for byte.
		"BS", "HT", "NL", "VT", "FF", "CR", "SO", "SI",
		"DLE", "DC1", "DC2", "DC3", "DC4", "NAK", "SYN", "ETB",
		"CAN", "EM", "SUB", "ESC", "FS", "GS", "RS", "US",
	}
	switch {
	case int(code) < len(names):
		return names[code]
	case code == 0x20:
		return ""
	case code == 0x7f:
		return "DEL"
	}
	return string(rune(code))
}
