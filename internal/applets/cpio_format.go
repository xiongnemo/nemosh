package applets

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// The cpio `newc` format, read and written here rather than through a library:
// Go has no cpio package, and the format is a fixed-width ASCII header followed
// by the name and the data, twice padded.
//
// A header is 110 bytes: a six-character magic and thirteen eight-character
// hexadecimal fields, upper case, zero-padded. Then the name including its
// terminating NUL, then enough padding that the *data* starts on a four-byte
// boundary counted from the start of the header -- and then the data, padded the
// same way. Both paddings are easy to get subtly wrong, so each is one function
// used by both directions.
//
// Only `newc` and its CRC variant are handled. busybox's own -o requires -H newc
// (its usage line says so), and the older octal and raw-binary headers are a
// second and third parser for archives nothing on this platform produces. They
// are refused by magic, with the magic quoted, rather than misread.

const (
	cpioHeaderSize = 110
	cpioMagicNewc  = "070701"
	// The CRC variant is the same layout with a real checksum in the last field.
	// It is read -- the layout is identical -- and never written.
	cpioMagicCRC = "070702"
	// cpioTrailer ends an archive. An archive without it is truncated, which is
	// worth saying rather than treating as a clean end.
	cpioTrailer = "TRAILER!!!"
)

// cpioEntry is one member: everything the header carries that this platform can
// act on, plus where the data is.
type cpioEntry struct {
	name  string
	mode  int64
	mtime int64
	size  int64
	// nlink and ino together are how a hardlink is spotted. Kept because a
	// listing that silently turns one file into two copies is a lie about the
	// archive, even though this build does not recreate the link.
	nlink int64
	ino   int64
	// uid and gid are carried only so that -tv can report what the archive says.
	// Nothing here acts on them: Windows has no numeric owner to set them on.
	uid int64
	gid int64
}

func (e cpioEntry) isDir() bool     { return e.mode&0o170000 == 0o040000 }
func (e cpioEntry) isSymlink() bool { return e.mode&0o170000 == 0o120000 }
func (e cpioEntry) isRegular() bool { return e.mode&0o170000 == 0o100000 }

// readCpioHeader reads one entry's header and name. It answers a nil entry at the
// trailer, which is the only clean end.
func readCpioHeader(reader io.Reader) (*cpioEntry, error) {
	// The magic is read on its own, before the rest of the header, so that an
	// archive in a format this build does not read is named rather than reported as
	// truncated. An odc header is 76 bytes and the old binary one is 26, so asking
	// for 110 first turns "I cannot read this format" into "unexpected EOF" for
	// every small archive in either of them.
	header := make([]byte, cpioHeaderSize)
	if _, err := io.ReadFull(reader, header[:6]); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("archive ends without a %s entry, so it is truncated", cpioTrailer)
		}
		return nil, fmt.Errorf("cannot read a cpio header: %v", err)
	}
	magic := string(header[:6])
	if magic != cpioMagicNewc && magic != cpioMagicCRC {
		return nil, fmt.Errorf("unsupported cpio format %q: only newc (%s) and its CRC variant are read",
			magic, cpioMagicNewc)
	}
	if _, err := io.ReadFull(reader, header[6:]); err != nil {
		return nil, fmt.Errorf("cannot read a cpio header: %v", err)
	}
	fields, err := cpioHeaderFields(header)
	if err != nil {
		return nil, err
	}
	nameSize := fields[11]
	if nameSize <= 0 || nameSize > 4096 {
		return nil, fmt.Errorf("cpio entry claims a %d-byte name", nameSize)
	}
	name := make([]byte, nameSize)
	if _, err := io.ReadFull(reader, name); err != nil {
		return nil, fmt.Errorf("cannot read a cpio entry name: %v", err)
	}
	if err := skipCpioPadding(reader, cpioHeaderSize+nameSize); err != nil {
		return nil, err
	}
	entry := &cpioEntry{
		// The NUL is part of the stored length and not part of the name.
		name:  strings.TrimRight(string(name), "\x00"),
		ino:   fields[0],
		mode:  fields[1],
		nlink: fields[4],
		mtime: fields[5],
		size:  fields[6],
		uid:   fields[2],
		gid:   fields[3],
	}
	if entry.name == cpioTrailer {
		return nil, nil
	}
	if entry.size < 0 {
		return nil, fmt.Errorf("cpio entry %s claims a negative size", entry.name)
	}
	return entry, nil
}

// cpioHeaderFields parses the thirteen hexadecimal fields after the magic.
func cpioHeaderFields(header []byte) ([13]int64, error) {
	var fields [13]int64
	for index := range fields {
		start := 6 + index*8
		text := string(header[start : start+8])
		value, err := strconv.ParseInt(strings.TrimSpace(text), 16, 64)
		if err != nil {
			return fields, fmt.Errorf("cpio header field %d is not hexadecimal: %q", index, text)
		}
		fields[index] = value
	}
	return fields, nil
}

// cpioPadding is how many bytes follow something of this length to reach the next
// four-byte boundary. Both the header-plus-name and the data are padded this way.
func cpioPadding(length int64) int64 {
	return (4 - length%4) % 4
}

func skipCpioPadding(reader io.Reader, length int64) error {
	padding := cpioPadding(length)
	if padding == 0 {
		return nil
	}
	if _, err := io.CopyN(io.Discard, reader, padding); err != nil {
		return fmt.Errorf("cannot skip cpio padding: %v", err)
	}
	return nil
}

// writeCpioEntry writes one header, name and body, with both paddings.
func writeCpioEntry(writer io.Writer, entry cpioEntry, body io.Reader) error {
	name := entry.name + "\x00"
	// uid and gid are written as zero rather than invented. Windows has no
	// numeric owner to report -- busybox-w32 writes 4095 for both, which is not a
	// user either -- and zero is the one value that reads as "not recorded"
	// wherever the archive is unpacked.
	fields := [13]int64{
		entry.ino, entry.mode, 0, 0, max(entry.nlink, 1), entry.mtime, entry.size,
		0, 0, 0, 0, int64(len(name)), 0,
	}
	header := cpioMagicNewc
	for _, value := range fields {
		header += fmt.Sprintf("%08X", uint32(value))
	}
	if _, err := io.WriteString(writer, header+name); err != nil {
		return err
	}
	if err := writeCpioPadding(writer, cpioHeaderSize+int64(len(name))); err != nil {
		return err
	}
	if body != nil {
		written, err := io.Copy(writer, body)
		if err != nil {
			return err
		}
		if written != entry.size {
			// The size is in a header already written, so a file that changed
			// under us would silently misalign every entry after it.
			return fmt.Errorf("%s changed size while being archived: %d bytes, header says %d",
				entry.name, written, entry.size)
		}
	}
	return writeCpioPadding(writer, entry.size)
}

func writeCpioPadding(writer io.Writer, length int64) error {
	padding := cpioPadding(length)
	if padding == 0 {
		return nil
	}
	_, err := writer.Write(make([]byte, padding))
	return err
}

// countingReader and countingWriter exist for one line of output: both references
// end with `N blocks` on stderr, where a block is 512 bytes and N covers the whole
// archive including the trailer. Reporting it means knowing the byte total, and the
// stream may be a pipe with nothing to measure afterwards.
type countingReader struct {
	inner io.Reader
	total int64
}

func (c *countingReader) Read(buffer []byte) (int, error) {
	read, err := c.inner.Read(buffer)
	c.total += int64(read)
	return read, err
}

type countingWriter struct {
	inner io.Writer
	total int64
}

func (c *countingWriter) Write(buffer []byte) (int, error) {
	written, err := c.inner.Write(buffer)
	c.total += int64(written)
	return written, err
}

// cpioBlocks rounds up, which is why 388 bytes reads as `1 blocks` -- including
// busybox's disagreeing noun, kept because a differential test compares strings.
func cpioBlocks(bytes int64) int64 {
	return (bytes + 511) / 512
}

// writeCpioTrailer ends the archive. Its absence is what readCpioHeader reports as
// a truncation, so writing it is not optional.
func writeCpioTrailer(writer io.Writer) error {
	return writeCpioEntry(writer, cpioEntry{name: cpioTrailer, nlink: 1}, nil)
}
