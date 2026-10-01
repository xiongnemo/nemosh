package applets

import (
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"strings"
)

// What gunzip, bunzip2 and zcat read, and what they say of data they cannot.

// compressFault is a fault in the data, worded as busybox's decompressors word it. It
// names no FILE: busybox names the one it cannot open, and not the one it cannot read,
// so `gunzip n.gz` says `gunzip: invalid magic`.
type compressFault string

func (e compressFault) Error() string { return string(e) }

const (
	faultMagic     compressFault = "invalid magic"
	faultCorrupted compressFault = "corrupted data"
	faultEnd       compressFault = "unexpected end of file"
)

var (
	gzipMagic  = []byte{0x1f, 0x8b}
	bzip2Magic = []byte("BZ")
	xzMagic    = []byte{0xfd, '7', 'z', 'X', 'Z', 0}
)

// decompressor reads input as codec's data. zcat's codec is "", and the data's first
// bytes choose it, as busybox's "clever zcat" has them: gzip's or bzip2's. A FILE is
// read whole with its name ignored, so `zcat a.bz2` is bzcat's answer.
func decompressor(codec string, input io.Reader) (io.Reader, error) {
	buffered := bufio.NewReader(input)
	if codec == "" {
		head, _ := buffered.Peek(len(xzMagic))
		switch {
		case len(head) < len(gzipMagic):
			return nil, compressFault("short read")
		case bytes.HasPrefix(head, gzipMagic):
			codec = "gzip"
		case bytes.HasPrefix(head, bzip2Magic):
			codec = "bzip2"
		case bytes.HasPrefix(head, xzMagic):
			return nil, compressFault("cannot decompress xz: gzip and bzip2 are the ones here")
		default:
			return nil, compressFault("no gzip/bzip2/xz magic")
		}
	}
	magic := gzipMagic
	if codec == "bzip2" {
		magic = bzip2Magic
	}
	if !startsWith(buffered, magic) {
		return nil, faultMagic
	}
	if codec == "bzip2" {
		return bunzip2Reader{bzip2.NewReader(buffered)}, nil
	}
	member, err := gzip.NewReader(buffered)
	if err != nil {
		return nil, faultCorrupted
	}
	member.Multistream(false)
	return &gunzipReader{input: buffered, member: member}, nil
}

// startsWith reports whether input's next bytes are magic, and leaves them to be read.
func startsWith(input *bufio.Reader, magic []byte) bool {
	head, _ := input.Peek(len(magic))
	return bytes.Equal(head, magic)
}

// gunzipReader reads gzip members one after another, as busybox's unpack_gz_stream does:
// another follows a member where gzip's magic does, and what else follows is passed over.
// Go's multistream reader read it as a header, and a gzip with a few bytes after it failed.
type gunzipReader struct {
	input  *bufio.Reader
	member *gzip.Reader
}

func (r *gunzipReader) Read(buffer []byte) (int, error) {
	for {
		n, err := r.member.Read(buffer)
		if err != io.EOF {
			return n, gunzipFault(err)
		}
		if n > 0 {
			return n, nil
		}
		if !startsWith(r.input, gzipMagic) {
			return 0, io.EOF
		}
		if r.member.Reset(r.input) != nil {
			return 0, faultCorrupted
		}
		r.member.Multistream(false)
	}
}

// gunzipFault words a member's fault as busybox's inflate does. Go's ErrChecksum stands
// for the length as well as the CRC, which busybox calls `incorrect length`.
func gunzipFault(err error) error {
	_, corrupt := errors.AsType[flate.CorruptInputError](err)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, io.ErrUnexpectedEOF):
		return faultEnd
	case errors.Is(err, gzip.ErrChecksum):
		return compressFault("crc error")
	case corrupt, errors.Is(err, gzip.ErrHeader):
		return faultCorrupted
	}
	return err
}

// bunzip2Reader is the standard library's bzip2 with busybox's words for its faults.
// Another stream follows one where "BZ" does, and what else follows is passed over, as
// busybox's unpack_bz2_stream has it. busybox says `bunzip error -3` and `-5`, its
// decoder's return codes, where the end comes early and where the data is bad; these are
// the words its gunzip has for the two.
type bunzip2Reader struct{ stream io.Reader }

func (r bunzip2Reader) Read(buffer []byte) (int, error) {
	n, err := r.stream.Read(buffer)
	structural, ok := errors.AsType[bzip2.StructuralError](err)
	switch {
	case err == nil, err == io.EOF:
		return n, err
	case errors.Is(err, io.ErrUnexpectedEOF):
		return n, faultEnd
	case !ok:
		return n, err
	case structural == "bad magic value in continuation file":
		return n, io.EOF
	case strings.HasSuffix(string(structural), "checksum mismatch"):
		return n, compressFault("CRC error")
	}
	return n, faultCorrupted
}
