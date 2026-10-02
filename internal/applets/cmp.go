package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
)

// cmp is busybox's (editors/cmp.c): `cmp [-l|s] [-n NUM] FILE1 [FILE2 [SKIP1 [SKIP2]]]`, the
// first byte where FILE1 and FILE2, or stdin, differ, `a b differ: byte 5, line 2`; with -l
// each one, its position and the two bytes in octal; with -s none, the status alone saying.
// SKIP1 and SKIP2 bytes are passed over first, K M and G after either counting in 1024s, and
// -n compares no more than NUM. A FILE that ends first is `cmp: EOF on FILE`, on stderr. The
// status is 0 for the same, 1 for different, and 2 for a FILE that cannot be read, which -s
// names no more than the rest.
//
// It said `char` where busybox and GNU both say `byte`, put the EOF on stdout, needed FILE2,
// took neither SKIP nor -n, and read -l and printed the first difference as if it had not.
func newCmpApplet() Applet {
	return simpleApplet{name: "cmp", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, paths, err := parseAppletLongOptions(ctx, args, map[string]string{"bytes": "n", "quiet": "s", "silent": "s", "verbose": "l"}, "sl", "n")
		if err != nil {
			return err
		}
		if options.has('l') && options.has('s') {
			return errors.New("options -l and -s are mutually exclusive")
		}
		limit := -1
		if options.has('n') {
			if limit, err = positiveNumber(options.value('n')); err != nil {
				return err
			}
		}
		if len(paths) == 0 {
			return missingOperand()
		}
		if len(paths) > 4 {
			return fmt.Errorf("extra operand '%s'", paths[4])
		}
		names := []string{paths[0], "-"}
		if len(paths) > 1 {
			names[1] = paths[1]
		}
		var skips [2]uint64
		for index, text := range paths[min(len(paths), 2):] {
			if skips[index], err = busyboxNumberBase(text, 10, math.MaxUint64, math.MaxUint64, kmgSuffixes); err != nil {
				return err
			}
		}
		// Both stdin: the same bytes, and busybox reads neither.
		if names[0] == "-" && names[1] == "-" {
			return nil
		}
		c := cmpRun{names: names, list: options.has('l'), quiet: options.has('s'), limit: limit}
		return c.run(ctx, stdin, stdout, stderr, skips)
	}}
}

// cmpRun is one comparison.
type cmpRun struct {
	names       []string
	list, quiet bool
	limit       int
}

func (c cmpRun) run(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, skips [2]uint64) error {
	view := ProcessViewFromContext(ctx)
	var inputs [2]*bufio.Reader
	for index, name := range c.names {
		file, err := OpenProcessOperand(ctx, view, name, stdin)
		if err != nil {
			if c.quiet {
				return ExitStatus(2)
			}
			return ExitStatusMessage(2, operandFailure(name, err))
		}
		defer file.Close()
		inputs[index] = bufio.NewReaderSize(file, 64<<10)
	}
	for index, input := range inputs {
		if _, err := io.CopyN(io.Discard, input, int64(min(skips[index], math.MaxInt64))); err != nil && !errors.Is(err, io.EOF) {
			return ExitStatusMessage(2, operandFailure(c.names[index], err))
		}
	}
	out := bufio.NewWriter(stdout)
	status, err := c.compare(inputs, out, stderr)
	if flushErr := out.Flush(); err == nil && flushErr != nil {
		return flushErr
	}
	switch {
	case err != nil:
		return ExitStatusMessage(2, err)
	case status != 0:
		return ExitStatus(status)
	}
	return nil
}

// compare is busybox's loop over the two: a byte from each in turn until one ends, or with
// neither -l nor another difference to list, at the first that differs.
func (c cmpRun) compare(inputs [2]*bufio.Reader, out *bufio.Writer, stderr io.Writer) (int, error) {
	status, line := 0, 1
	for position, limit := uint64(1), c.limit; limit != 0; position, limit = position+1, limit-1 {
		first, firstErr := inputs[0].ReadByte()
		second, secondErr := inputs[1].ReadByte()
		if firstErr != nil && !errors.Is(firstErr, io.EOF) {
			return 2, operandFailure(c.names[0], firstErr)
		}
		if secondErr != nil && !errors.Is(secondErr, io.EOF) {
			return 2, operandFailure(c.names[1], secondErr)
		}
		if firstErr == nil && secondErr == nil && first == second {
			if first == '\n' {
				line++
			}
			continue
		}
		if firstErr != nil && secondErr != nil {
			return status, nil
		}
		status = 1
		if firstErr != nil || secondErr != nil {
			// One ended first, busybox's fmt_eof, which it writes to stderr once what -l listed
			// is out.
			if !c.quiet {
				ended := c.names[0]
				if secondErr != nil {
					ended = c.names[1]
				}
				if err := out.Flush(); err != nil {
					return 2, err
				}
				fmt.Fprintf(stderr, "cmp: EOF on %s\n", ended)
			}
			return status, nil
		}
		switch {
		case c.quiet:
			return status, nil
		case c.list:
			fmt.Fprintf(out, "%d %3o %3o\n", position, first, second)
		default:
			_, err := fmt.Fprintf(out, "%s %s differ: byte %d, line %d\n", c.names[0], c.names[1], position, line)
			return status, err
		}
	}
	return status, nil
}
