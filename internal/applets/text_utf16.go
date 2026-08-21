package applets

import (
	"bufio"
	"context"
	"io"
	"unicode/utf16"
	"unicode/utf8"
)

// Reading the UTF-16 that Windows writes.
//
// Notepad's "Unicode", PowerShell 5.1's `>` redirection and every registry export produce UTF-16LE
// with a byte-order mark, and until this existed `grep hello` over one of those files found nothing.
// Not a wrong answer exactly -- the bytes really do not contain the ASCII `hello` -- but not an
// answer anybody wanted, and silence is the failure mode this project likes least. busybox-w32 does
// not read them either, so this is a feature beyond the reference rather than a divergence from it.
//
// Three decisions, and the first two are refusals.
//
// **A byte-order mark, and nothing else.** No counting NUL bytes, no sniffing for a pattern of odd
// and even zeroes. A heuristic that decides an encoding for a file that never declared one is a
// heuristic that will eventually decide wrongly about a binary, and rewriting a binary is worse than
// declining to read a text file. A BOM is the writer saying what it wrote; that can be trusted.
// UTF-16 without a BOM stays unread, which is also where ripgrep draws this line.
//
// **Only applets that interpret text.** `cat`, `head`, `tail`, `base64` and the rest stay byte-exact,
// because `cat a > b` has to copy a file rather than reinterpret it -- the property internal/applets/
// text_encoding.go already protects for GBK. Decoding belongs to the applets that must understand
// the characters to do their job at all: a regular expression cannot match across UTF-16 code units,
// and a character count of a UTF-16 file is not a byte count.
//
// **Output is UTF-8.** A decoded line printed back comes out as UTF-8, which is the only choice that
// does not require every applet to remember what it read. It does mean `grep x u16.txt > out.txt`
// writes UTF-8, and that is worth knowing rather than hiding; see docs/support-matrix.md.

// byteOrderMarks are the three this recognises, longest first so UTF-8's three bytes are tested
// before anything shorter could match a prefix of them.
var byteOrderMarks = []struct {
	prefix []byte
	// bigEndian is meaningless for UTF-8, where the mark is only a mark.
	bigEndian bool
	utf16     bool
}{
	{prefix: []byte{0xEF, 0xBB, 0xBF}},
	{prefix: []byte{0xFE, 0xFF}, bigEndian: true, utf16: true},
	{prefix: []byte{0xFF, 0xFE}, utf16: true},
}

// decodeTextInput returns a reader over the same text as UTF-8, when a byte-order mark says what the
// bytes are. Anything else is returned unchanged, including its first bytes.
//
// The mark itself is consumed either way. A UTF-8 BOM is a zero-width space as far as a regular
// expression is concerned, so leaving it in makes `grep '^hello'` fail on the first line of a file
// Notepad wrote -- which is the same defect as not decoding UTF-16, in a form people meet more often.
func decodeTextInput(input io.Reader) io.Reader {
	reader := bufio.NewReader(input)
	for _, mark := range byteOrderMarks {
		prefix, err := reader.Peek(len(mark.prefix))
		if err != nil || !equalBytes(prefix, mark.prefix) {
			continue
		}
		_, _ = reader.Discard(len(mark.prefix))
		if !mark.utf16 {
			return reader
		}
		return &utf16Reader{source: reader, bigEndian: mark.bigEndian}
	}
	return reader
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// utf16Reader converts UTF-16 code units to UTF-8 as they are read.
//
// Streaming rather than reading the file in, because these applets are expected to work on something
// larger than memory -- `grep` over a log is the case that matters. So the awkward parts are the
// boundaries: a read can end between the two bytes of a code unit, and it can end between the two
// halves of a surrogate pair. Both are held over to the next read rather than decoded early, which is
// what stops an emoji splitting into two replacement characters at a buffer boundary.
type utf16Reader struct {
	source    *bufio.Reader
	bigEndian bool
	// pending is decoded UTF-8 not yet handed to the caller.
	pending []byte
	// half is one leftover byte of a code unit, and surrogate a leftover high surrogate.
	half      []byte
	surrogate rune
	err       error
}

func (r *utf16Reader) Read(out []byte) (int, error) {
	for len(r.pending) == 0 {
		if r.err != nil {
			// A trailing high surrogate with nothing to pair it with, or an odd final
			// byte: emit the replacement character rather than dropping bytes silently.
			if r.surrogate != 0 || len(r.half) > 0 {
				r.surrogate, r.half = 0, nil
				r.pending = append(r.pending, string(utf8.RuneError)...)
				break
			}
			return 0, r.err
		}
		r.fill()
	}
	written := copy(out, r.pending)
	r.pending = r.pending[written:]
	return written, nil
}

// fill reads a chunk and decodes as much of it as forms whole characters.
func (r *utf16Reader) fill() {
	buffer := make([]byte, 4096)
	read, err := r.source.Read(buffer)
	if err != nil {
		r.err = err
	}
	if read == 0 {
		return
	}
	data := append(r.half, buffer[:read]...)
	r.half = nil
	if odd := len(data) % 2; odd != 0 {
		r.half = append(r.half, data[len(data)-1])
		data = data[:len(data)-1]
	}
	units := make([]uint16, 0, len(data)/2+1)
	if r.surrogate != 0 {
		units = append(units, uint16(r.surrogate))
		r.surrogate = 0
	}
	for index := 0; index+1 < len(data); index += 2 {
		if r.bigEndian {
			units = append(units, uint16(data[index])<<8|uint16(data[index+1]))
			continue
		}
		units = append(units, uint16(data[index+1])<<8|uint16(data[index]))
	}
	// A high surrogate at the very end has its pair in the next chunk. utf16.Decode would
	// turn it into a replacement character here, so it waits.
	if len(units) > 0 && utf16.IsSurrogate(rune(units[len(units)-1])) {
		last := units[len(units)-1]
		if last >= 0xD800 && last <= 0xDBFF {
			r.surrogate = rune(last)
			units = units[:len(units)-1]
		}
	}
	r.pending = append(r.pending, []byte(string(utf16.Decode(units)))...)
}

// openProcessTextInput opens a named file for an applet that interprets what it reads.
//
// Named apart from OpenProcessInput so the choice is visible at every call site: an applet either
// interprets its input, in which case an encoding it can recognise should be decoded, or it moves
// bytes, in which case it must not touch them.
func openProcessTextInput(ctx context.Context, view ProcessView, path string) (io.ReadCloser, error) {
	file, err := OpenProcessInput(ctx, view, path)
	if err != nil {
		return nil, err
	}
	return decodedCloser{Reader: decodeTextInput(file), closer: file}, nil
}

type decodedCloser struct {
	io.Reader
	closer io.Closer
}

func (c decodedCloser) Close() error { return c.closer.Close() }
