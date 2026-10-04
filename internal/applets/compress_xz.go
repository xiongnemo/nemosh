package applets

import (
	"bufio"
	"errors"
	"io"

	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

// unxz, xzcat, unlzma and lzcat, and xz and lzma with -d: they read through
// ulikunitz/xz, and they only read, as busybox's do, whose xz and lzma without -d answer
// with their usage. tar writes both all the same, -cJ and -c --lzma, which busybox's tar
// does through an xz program it finds on PATH; here the encoders are in the binary.
//
// What busybox says of data they cannot read is said here too: an xz stream cut short or
// damaged is `corrupted data`, and an lzma header that will not read is `bad lzma header`.
// Data that is not xz at all busybox's decoder reads as nothing and exits 0, which this one
// does not: it is `invalid magic`, as gunzip says it of what is not gzip.

// xzDecompressor reads xz's data, magic first.
func xzDecompressor(input *bufio.Reader) (io.Reader, error) {
	if !startsWith(input, xzMagic) {
		return nil, faultMagic
	}
	end := &xzEnd{reader: input}
	reader, err := xz.NewReader(end)
	if err != nil {
		return nil, faultCorrupted
	}
	return xzFaults{reader: reader, end: end}, nil
}

// xzEnd watches what the decoder reads for how it ends. A whole stream ends with its footer's
// magic, YZ, and nothing after it but zeros, the padding between streams. The decoder took a
// stream cut off in a block's header for an empty one and said nothing, where busybox's says
// corrupted data.
type xzEnd struct {
	reader io.Reader
	// last is the last byte that was not a zero and the byte before it; previous is the last
	// byte of all.
	last     [2]byte
	previous byte
}

func (e *xzEnd) Read(p []byte) (int, error) {
	n, err := e.reader.Read(p)
	for _, b := range p[:n] {
		if b != 0 {
			e.last = [2]byte{e.previous, b}
		}
		e.previous = b
	}
	return n, err
}

func (e *xzEnd) whole() bool { return e.last == [2]byte{'Y', 'Z'} }

// lzmaDecompressor reads the lzma format, which has no magic, only a header.
func lzmaDecompressor(input *bufio.Reader) (io.Reader, error) {
	reader, err := lzma.NewReader(input)
	if err != nil {
		return nil, compressFault("bad lzma header")
	}
	return xzFaults{reader: reader}, nil
}

// xzFaults is a decoder whose every failure is busybox's word for it, corrupted data, and an
// xz stream's end one only where the stream is whole.
type xzFaults struct {
	reader io.Reader
	end    *xzEnd
}

func (r xzFaults) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	switch {
	case err == nil:
	case !errors.Is(err, io.EOF), r.end != nil && !r.end.whole():
		return n, faultCorrupted
	}
	return n, err
}

// xzCompressor is tar's encoder for -J and --lzma.
func xzCompressor(out io.Writer, codec string) (io.WriteCloser, error) {
	if codec == "lzma" {
		return lzma.NewWriter(out)
	}
	return xz.NewWriter(out)
}
