package applets

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// xxd: bytes, looked at.

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
		options, paths, err := parseAppletOptions(ctx, args, "p", "")
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
