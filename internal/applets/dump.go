package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// xxd and split: bytes, looked at and cut up.

// xxd writes a hex dump.
//
// The layout is xxd's own, measured to the column:
//
//	00000000: 6865 6c6c 6f20 776f 726c 642c 2074 6869  hello world, thi
//	00000020: 6e65 2066 6f72 2078 7864 0a              ne for xxd.
//
// Eight-digit offset, colon, space; sixteen bytes as eight space-separated pairs;
// two spaces; then the printable column with a dot for anything else. A short
// final line is padded so the text column still lines up -- which is the part an
// implementation gets wrong first, and the reason the dump is worth reading at
// all.
func newXxdApplet() Applet {
	return simpleApplet{name: "xxd", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "p", "")
		if err != nil {
			return err
		}
		return eachTextInput(ctx, paths, stdin, func(reader io.Reader) error {
			data, err := io.ReadAll(reader)
			if err != nil {
				return err
			}
			if options.has('p') {
				return writePlainHex(stdout, data)
			}
			return writeHexDump(stdout, data)
		})
	}}
}

const xxdColumns = 16

func writeHexDump(stdout io.Writer, data []byte) error {
	for offset := 0; offset < len(data); offset += xxdColumns {
		end := min(offset+xxdColumns, len(data))
		chunk := data[offset:end]
		var hexPart strings.Builder
		for index := range xxdColumns {
			if index < len(chunk) {
				fmt.Fprintf(&hexPart, "%02x", chunk[index])
			} else {
				// Spaces where a byte would have been, so the text column of a
				// short last line stays under the one above it.
				hexPart.WriteString("  ")
			}
			if index%2 == 1 && index != xxdColumns-1 {
				hexPart.WriteByte(' ')
			}
		}
		if _, err := fmt.Fprintf(stdout, "%08x: %s  %s\n", offset, hexPart.String(), printableColumn(chunk)); err != nil {
			return err
		}
	}
	return nil
}

// printableColumn is the right-hand side: a byte that would move the cursor or
// mean nothing on screen becomes a dot, which is what makes the column readable
// rather than a source of stray escapes.
func printableColumn(chunk []byte) string {
	var text strings.Builder
	for _, b := range chunk {
		if b < 0x20 || b > 0x7e {
			text.WriteByte('.')
			continue
		}
		text.WriteByte(b)
	}
	return text.String()
}

func writePlainHex(stdout io.Writer, data []byte) error {
	var hexOnly strings.Builder
	for _, b := range data {
		fmt.Fprintf(&hexOnly, "%02x", b)
	}
	_, err := fmt.Fprintln(stdout, hexOnly.String())
	return err
}

// split cuts a file into pieces.
//
//	$ split -l 2 sp part_   ->  part_aa, part_ab
//
// The suffix is the interesting part: two letters, counting `aa`, `ab`, ... which
// is what makes the pieces sort back into order. Running out of suffixes is an
// error rather than a wrap, because `part_aa` appearing twice would silently lose
// data.
func newSplitApplet() Applet {
	return simpleApplet{name: "split", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "", "l")
		if err != nil {
			return err
		}
		lines := 1000
		if options.has('l') {
			if lines, err = strconv.Atoi(options.value('l')); err != nil || lines <= 0 {
				return fmt.Errorf("invalid number of lines: %s", options.value('l'))
			}
		}
		source := "-"
		prefix := "x"
		if len(paths) > 0 {
			source = paths[0]
		}
		if len(paths) > 1 {
			prefix = paths[1]
		}
		if len(paths) > 2 {
			return fmt.Errorf("extra operand '%s'", paths[2])
		}
		view := ProcessViewFromContext(ctx)
		content, err := readOperandLines(ctx, view, source, stdin)
		if err != nil {
			return err
		}
		return writeSplitParts(ctx, view, prefix, content, lines)
	}}
}

func writeSplitParts(ctx context.Context, view ProcessView, prefix string, lines []string, perPart int) error {
	for start, part := 0, 0; start < len(lines); start, part = start+perPart, part+1 {
		// Once per part, which is where the interruptible boundary actually is.
		//
		// Reading the input is already cancellable and writing one part is bounded, but
		// the *loop* is not bounded by anything a person can see: `split -l 1` over a
		// million lines writes a million files, and Ctrl-C could not stop it. The context
		// was already being passed down to the write, which ignored it -- carrying a
		// context you do not honour advertises cancellation you do not provide.
		if err := ctx.Err(); err != nil {
			return err
		}
		suffix, err := splitSuffix(part)
		if err != nil {
			return err
		}
		end := min(start+perPart, len(lines))
		body := strings.Join(lines[start:end], "\n") + "\n"
		if err := writeProcessFile(view, prefix+suffix, body); err != nil {
			return err
		}
	}
	return nil
}

// splitSuffix is `aa`, `ab`, ... `zz`. Beyond that it is an error: GNU grows the
// suffix, and growing it would make the pieces stop sorting into order, which is
// the one property the naming exists for.
func splitSuffix(part int) (string, error) {
	const letters = 26
	if part >= letters*letters {
		return "", fmt.Errorf("output file suffixes exhausted")
	}
	return string([]byte{byte('a' + part/letters), byte('a' + part%letters)}), nil
}

// writeProcessFile creates a file named the way the shell names it, crossing the
// path seam once. os.Create on the operand as given would write to the host's
// idea of `/c/...`, which is a real path in the wrong place.
func writeProcessFile(view ProcessView, path, body string) error {
	native, err := resolveHostPath(view, path)
	if err != nil {
		return err
	}
	file, err := os.Create(native)
	if err != nil {
		return cannotOpen(path, err)
	}
	_, writeErr := io.WriteString(file, body)
	return errors.Join(writeErr, file.Close())
}
