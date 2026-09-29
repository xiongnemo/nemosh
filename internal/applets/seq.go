package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// seq is busybox's (coreutils/seq.c): `seq [-w] [-s SEP] [FIRST [INC]] LAST`. The operands are
// C doubles, as strtod reads them, and each number printed is FIRST plus a whole number of INCs,
// so the steps do not drift. They are printed with as many decimals as the most any operand but
// LAST was written with, and under -w padded with zeros to the widest operand's integer part;
// -s SEP goes between them where a newline did. A word that is a negative number ends the
// options, so `seq -1 1` counts from -1.
//
// It read integers alone, so `seq 1 0.5 2` was refused, and took no options, so `seq -w 8 10`
// and `seq -s, 3` were bad numbers. A zero INC is still refused, where busybox counts forever.
func newSeqApplet() Applet {
	return simpleApplet{name: "seq", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		split := len(args)
		for index, arg := range args {
			if len(arg) > 1 && arg[0] == '-' && (arg[1] == '.' || isASCIIDigit(arg[1])) {
				split = index
				break
			}
		}
		options, operands, err := parseAppletOptionsInOrder(args[:split], "w", "s")
		if err != nil {
			return err
		}
		operands = append(operands, args[split:]...)
		switch {
		case len(operands) == 0:
			return missingOperand()
		case len(operands) > 3:
			return fmt.Errorf("extra operand '%s'", operands[3])
		}
		values := make([]float64, len(operands))
		for index, operand := range operands {
			value, end := cNumber(operand, true)
			if end != len(operand) {
				return fmt.Errorf("invalid number: %s", operand)
			}
			values[index] = value
		}
		first, increment, last := 1.0, 1.0, values[len(values)-1]
		if len(values) > 1 {
			first = values[0]
		}
		if len(values) == 3 {
			increment = values[1]
		}
		if increment == 0 {
			return errors.New("invalid increment: 0")
		}
		width, decimals := seqFormat(operands, options.has('w'))
		separator := "\n"
		if options.has('s') {
			separator = options.value('s')
		}
		writer := bufio.NewWriter(stdout)
		count := 0
		for ; ; count++ {
			value := first + float64(count)*increment
			more := value >= last
			if increment >= 0 {
				more = value <= last
			}
			if !more {
				break
			}
			if count%4096 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			if count > 0 {
				writer.WriteString(separator)
			}
			// A reader that has gone, `seq inf | head -1`, ends it.
			if _, err := fmt.Fprintf(writer, "%0*.*f", width, decimals, value); err != nil {
				return err
			}
		}
		if count > 0 {
			writer.WriteString("\n")
		}
		return writer.Flush()
	}}
}

// seqFormat is busybox's width and decimals: the widest integer part of any operand, and the
// most decimals of any but LAST, which coreutils never looks at either; a width only under -w.
func seqFormat(operands []string, padded bool) (int, int) {
	width, fraction := 0, 0
	for index, operand := range operands {
		integer := strings.IndexByte(operand, '.')
		if integer < 0 {
			integer = len(operand)
		}
		width = max(width, integer)
		if index < len(operands)-1 {
			fraction = max(fraction, len(operand)-integer)
		}
	}
	// fraction counted the point.
	if fraction > 0 {
		if fraction--; fraction > 0 {
			width += fraction + 1
		}
	}
	if !padded {
		width = 0
	}
	return width, fraction
}
