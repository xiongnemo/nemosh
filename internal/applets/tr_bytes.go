package applets

import (
	"context"
	"io"
)

// trBytes is a translation as three tables of 256, for sets that hold only ASCII: what each byte
// becomes, whether it is deleted, and whether a run of it is squeezed. With ASCII sets a byte of
// 0x80 and up -- a piece of a UTF-8 character, or a byte that is not UTF-8 -- is never matched, so
// it passes as it is and breaks a squeezed run, exactly as the rune path passes its character.
// `tr a-z A-Z` of 92 MB took 2.23 s a rune at a time, looking each one up in its set in turn,
// four times busybox's 0.55 s, whose tr is a byte table too.
type trBytes struct {
	mapped  [256]byte
	drop    [256]bool
	squeeze [256]bool
}

// byteTable is t as tables of bytes, when it can be one: every character of both sets ASCII,
// and the first set not complemented, since a complement holds every character but a few, and
// the rune path takes a multi-byte one as one character where bytes would take it as several.
func (t trTable) byteTable() (*trBytes, bool) {
	if t.complement {
		return nil, false
	}
	for _, set := range [][]rune{t.from, t.to, t.squeezeSet} {
		for _, r := range set {
			if r >= 0x80 {
				return nil, false
			}
		}
	}
	var table trBytes
	for c := range 256 {
		table.mapped[c] = byte(c)
	}
	for c := range rune(0x80) {
		mapped, keep := t.apply(c)
		table.mapped[c], table.drop[c] = byte(mapped), !keep
		table.squeeze[c] = t.squeeze && t.squeezes(c)
	}
	return &table, true
}

// runBytes is run through a byte table, 64 KiB at a time.
func (t trTable) runBytes(ctx context.Context, stdout io.Writer, stdin io.Reader, table *trBytes) error {
	writer := newFilterOutput(stdout)
	reader := writer.input(contextReader{ctx: ctx, reader: stdin})
	in, out := make([]byte, 64<<10), make([]byte, 0, 64<<10)
	previous := -1
	for {
		n, err := reader.Read(in)
		out = out[:0]
		for _, c := range in[:n] {
			if table.drop[c] {
				previous = -1
				continue
			}
			mapped := table.mapped[c]
			if table.squeeze[mapped] && int(mapped) == previous {
				continue
			}
			previous = int(mapped)
			out = append(out, mapped)
		}
		if _, writeErr := writer.Write(out); writeErr != nil {
			return writeErr
		}
		if err != nil {
			if err == io.EOF {
				return writer.Flush()
			}
			return err
		}
	}
}
