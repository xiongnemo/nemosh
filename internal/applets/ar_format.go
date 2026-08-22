package applets

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// The ar format: an eight-byte magic, then fixed-width ASCII headers of sixty
// bytes each, each followed by its data padded to an even length.
//
// A header is text in fixed columns -- name, mtime, uid, gid, mode, size, and a
// two-byte end marker. Every numeric field is *decimal* except the mode, which is
// octal, and getting that backwards produces plausible nonsense rather than an
// error, so each is parsed with its base named.
//
// Names are the one complicated part. GNU terminates a short name with `/`, so
// `a.txt` is stored as `a.txt/`; a name too long for sixteen columns is stored as
// `/OFFSET` pointing into a member called `//`. Two special members are index
// rather than content: `/` is the symbol table and `//` is that long-name table.

const (
	arMagic       = "!<arch>\n"
	arHeaderSize  = 60
	arEndMarker   = "`\n"
	arNameColumns = 16
)

type arMember struct {
	// name as stored, which may be a `/OFFSET` reference rather than a name.
	name  string
	mtime int64
	uid   int64
	gid   int64
	mode  int64
	size  int64
}

func readArMagic(reader io.Reader) error {
	magic := make([]byte, len(arMagic))
	if _, err := io.ReadFull(reader, magic); err != nil {
		return fmt.Errorf("cannot read as an ar archive")
	}
	if string(magic) != arMagic {
		return fmt.Errorf("cannot read as an ar archive: it does not begin with %q", arMagic)
	}
	return nil
}

// readArHeader reads one member header, answering nil at a clean end of file.
//
// Unlike cpio there is no trailer: the archive simply stops, so EOF exactly at a
// header boundary is the normal end and EOF part-way through one is corruption.
func readArHeader(reader io.Reader) (*arMember, error) {
	header := make([]byte, arHeaderSize)
	read, err := io.ReadFull(reader, header)
	if err == io.EOF {
		return nil, nil
	}
	if err != nil {
		// A single stray newline is tolerated: some writers pad the last member
		// with one and then stop, which io.ReadFull reports as a short read.
		if read == 1 && header[0] == '\n' {
			return nil, nil
		}
		return nil, fmt.Errorf("the ar archive ends part-way through a header")
	}
	if marker := string(header[58:60]); marker != arEndMarker {
		return nil, fmt.Errorf("ar member header is malformed: it ends %q, not %q", marker, arEndMarker)
	}
	member := &arMember{name: strings.TrimRight(string(header[:arNameColumns]), " ")}
	for _, field := range []struct {
		text   string
		base   int
		target *int64
		what   string
	}{
		{string(header[16:28]), 10, &member.mtime, "mtime"},
		{string(header[28:34]), 10, &member.uid, "uid"},
		{string(header[34:40]), 10, &member.gid, "gid"},
		// Octal, and the only field that is: a mode written as decimal reads as a
		// different, entirely plausible mode.
		{string(header[40:48]), 8, &member.mode, "mode"},
		{string(header[48:58]), 10, &member.size, "size"},
	} {
		text := strings.TrimSpace(field.text)
		if text == "" {
			// uid, gid and mtime are routinely blank in a reproducible build.
			continue
		}
		value, err := strconv.ParseInt(text, field.base, 64)
		if err != nil {
			return nil, fmt.Errorf("ar member %s has an unreadable %s: %q", member.name, field.what, text)
		}
		*field.target = value
	}
	if member.size < 0 {
		return nil, fmt.Errorf("ar member %s claims a negative size", member.name)
	}
	return member, nil
}

// resolveArName turns the stored name into the real one.
//
// The trailing `/` is GNU's terminator and not part of the name. A stored `/N` is
// an offset into the long-name table, whose entries end in `/\n` or `\n`.
func resolveArName(stored, longNames string) (string, error) {
	if stored == "/" || stored == "//" {
		return stored, nil
	}
	if strings.HasPrefix(stored, "/") {
		offset, err := strconv.Atoi(strings.TrimSuffix(stored[1:], "/"))
		if err != nil {
			return "", fmt.Errorf("ar member name %q is not an offset into the long-name table", stored)
		}
		if offset < 0 || offset >= len(longNames) {
			return "", fmt.Errorf("ar member name points to offset %d, past the %d-byte long-name table",
				offset, len(longNames))
		}
		name, _, _ := strings.Cut(longNames[offset:], "\n")
		return strings.TrimSuffix(strings.TrimRight(name, " "), "/"), nil
	}
	return strings.TrimSuffix(stored, "/"), nil
}

// arPadding is one byte after an odd-length member and nothing after an even one.
func arPadding(size int64) int64 { return size % 2 }

func skipArPadding(reader io.Reader, size int64) error {
	if arPadding(size) == 0 {
		return nil
	}
	if _, err := io.CopyN(io.Discard, reader, 1); err != nil && err != io.EOF {
		return fmt.Errorf("cannot skip ar padding: %v", err)
	}
	return nil
}

func skipArBody(reader io.Reader, member arMember) error {
	if member.size > 0 {
		if _, err := io.CopyN(io.Discard, reader, member.size); err != nil {
			return fmt.Errorf("cannot skip ar member %s: %v", member.name, err)
		}
	}
	return skipArPadding(reader, member.size)
}

func readArBody(reader io.Reader, member arMember) ([]byte, error) {
	body := make([]byte, member.size)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, fmt.Errorf("cannot read ar member %s: %v", member.name, err)
	}
	return body, skipArPadding(reader, member.size)
}

// writeArHeader writes one member's fixed-width header.
//
// Only short names are written: a name that does not fit is refused rather than
// silently truncated, and the long-name table is not produced. Writing one
// correctly means every offset in it must be right or the archive is subtly wrong,
// and this build's reason for having `ar` is reading a `.deb`, whose four member
// names are all short.
func writeArHeader(writer io.Writer, member arMember) error {
	stored := member.name + "/"
	if len(stored) > arNameColumns {
		return fmt.Errorf("member name %q is too long for an ar header; this build does not write the long-name table", member.name)
	}
	header := fmt.Sprintf("%-16s%-12d%-6d%-6d%-8o%-10d%s",
		stored, member.mtime, member.uid, member.gid, member.mode, member.size, arEndMarker)
	if len(header) != arHeaderSize {
		// Any name or number that overflows its column would shift every field
		// after it, so the width is asserted rather than assumed.
		return fmt.Errorf("built a %d-byte ar header for %s, want %d", len(header), member.name, arHeaderSize)
	}
	_, err := io.WriteString(writer, header)
	return err
}

func writeArPadding(writer io.Writer, size int64) error {
	if arPadding(size) == 0 {
		return nil
	}
	// A newline rather than a NUL, which is what both references pad with.
	_, err := io.WriteString(writer, "\n")
	return err
}

// arModeString is the permission column `ar tv` prints. There is no type
// character: every member of an ar archive is a plain file.
func arModeString(mode int64) string {
	return modeString(cpioEntry{mode: 0o100000 | mode})[1:]
}

func arListingTime(mtime int64) string {
	if mtime == 0 {
		return strings.Repeat(" ", len("Jan  2 15:04 2006"))
	}
	return time.Unix(mtime, 0).Format("Jan _2 15:04 2006")
}
