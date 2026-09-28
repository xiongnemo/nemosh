package applets

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// od, hexdump and hd: three names over one dumper.
//
// `xxd` already exists here, so the shapes are pinned by differential rather than
// invented. The two families differ in their defaults, which is the whole reason
// both exist:
//
//	od       octal offsets, 16-bit words in octal
//	hexdump  hex offsets, 16-bit words in hex
//	hd       hexdump -C: hex bytes with an ASCII gutter
//
// The word forms read each pair of bytes **little-endian**, which is what makes
// `he` print as 6568 rather than 6865 and is the single most surprising thing
// about either tool.

func newOdApplet() Applet { return newDumperApplet("od") }

func newHexdumpApplet() Applet { return newDumperApplet("hexdump") }

func newHdApplet() Applet { return newDumperApplet("hd") }

// dumpFormat is what one line of output looks like.
type dumpFormat byte

const (
	dumpOctalWords dumpFormat = iota
	dumpHexWords
	dumpCanonical
	dumpChars
	dumpHexBytes
)

func newDumperApplet(name string) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "bcCdoxv", "At")
		if err != nil {
			return err
		}
		request := dumpRequest{format: defaultDumpFormat(name), radix: 'X'}
		if name == "od" {
			request.radix = 'o'
		}
		if err := request.apply(name, options); err != nil {
			return err
		}
		return eachTextFile(ctx, paths, stdin, func(reader io.Reader) error {
			return request.write(stdout, reader)
		})
	}}
}

func defaultDumpFormat(name string) dumpFormat {
	switch name {
	case "hd":
		return dumpCanonical
	case "hexdump":
		return dumpHexWords
	}
	return dumpOctalWords
}

type dumpRequest struct {
	format dumpFormat
	// more is the formats after the first, each a line of its own; see dumpBodies.
	more []dumpFormat
	// radix is the address's: 'o', 'd' or 'x' as -A names them, 'n' for none, and 'X' for
	// hexdump's own seven hex digits.
	radix byte
}

func (r *dumpRequest) apply(name string, options appletOptions) error {
	if options.has('C') {
		r.format = dumpCanonical
	}
	if options.has('c') {
		r.format = dumpChars
	}
	if options.has('x') {
		r.format = dumpHexWords
	}
	if options.has('o') {
		r.format = dumpOctalWords
	}
	if options.has('A') {
		// -A n suppresses the address entirely, which is what makes `od -An -tx1`
		// the usual way to get a bare hex stream. -A d was hex, and -A x seven digits
		// where busybox and GNU print six.
		value := options.value('A')
		switch value {
		case "n", "o", "d", "x":
			r.radix = value[0]
		default:
			return fmt.Errorf("invalid address radix '%s'", value)
		}
	}
	// Each -t is a format of its own, after any the letters asked for, as in both: only the
	// last one was used.
	var formats []dumpFormat
	if options.has('C') || options.has('c') || options.has('x') || options.has('o') {
		formats = append(formats, r.format)
	}
	for _, value := range options.all('t') {
		format, ok := map[string]dumpFormat{"x1": dumpHexBytes, "x": dumpHexBytes, "c": dumpChars, "a": dumpChars, "o2": dumpOctalWords, "o": dumpOctalWords, "x2": dumpHexWords}[value]
		if !ok {
			return fmt.Errorf("invalid type string '%s'", value)
		}
		formats = append(formats, format)
	}
	if len(formats) > 0 {
		r.format, r.more = formats[0], formats[1:]
	}
	return nil
}

// write dumps the whole input, sixteen bytes to a line.
func (r dumpRequest) write(stdout io.Writer, reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	for offset := 0; offset < len(data); offset += 16 {
		end := min(offset+16, len(data))
		// Not trimmed: hexdump's word form pads its line out to eight slots and
		// od's does not, so the padding is part of the body rather than something
		// to tidy away here. Measured against both.
		address := r.address(offset)
		for index, body := range r.bodies(data[offset:end]) {
			if index > 0 {
				address = strings.Repeat(" ", len(address))
			}
			if _, err := fmt.Fprintln(stdout, address+body); err != nil {
				return err
			}
		}
	}
	// The final line is the length, which is how a reader knows where the dump
	// stopped without counting the rows. With -A n there is no address to give, and so
	// no line, as in busybox: it was an empty one, which every `... | od -A n -c` then
	// carried into whatever read it.
	if r.radix == 'n' {
		return nil
	}
	if _, err := fmt.Fprintln(stdout, strings.TrimSpace(r.address(len(data)))); err != nil {
		return err
	}
	return nil
}

func (r dumpRequest) address(offset int) string {
	if r.format == dumpCanonical && r.radix != 'n' {
		return fmt.Sprintf("%08x  ", offset)
	}
	// No trailing space: every body form supplies its own separator, because the
	// character form's field is four wide *including* it. Adding one here put an
	// extra space before every `od -c` line.
	switch r.radix {
	case 'n':
		return ""
	case 'o':
		return fmt.Sprintf("%07o", offset)
	case 'd':
		return fmt.Sprintf("%07d", offset)
	case 'x':
		return fmt.Sprintf("%06x", offset)
	}
	return fmt.Sprintf("%07x", offset)
}

// bodies is a chunk as each format shows it, a line apiece. When every format shows a byte to
// a field, each field is as wide as the widest format's, so the lines stand in columns --
// `-t c -t x1` puts each byte's number under its character -- as GNU od has them.
func (r dumpRequest) bodies(chunk []byte) []string {
	formats := append([]dumpFormat{r.format}, r.more...)
	width := 0
	for _, format := range formats {
		switch format {
		case dumpChars:
			width = max(width, 4)
		case dumpHexBytes:
			width = max(width, 3)
		default:
			width = -1
		}
		if width < 0 {
			break
		}
	}
	bodies := make([]string, 0, len(formats))
	for _, format := range formats {
		if width < 0 || len(formats) == 1 {
			bodies = append(bodies, dumpBody(format, chunk))
			continue
		}
		var out strings.Builder
		for _, b := range chunk {
			field := dumpCharName(b)
			if format == dumpHexBytes {
				field = fmt.Sprintf("%02x", b)
			}
			fmt.Fprintf(&out, "%*s", width, field)
		}
		bodies = append(bodies, out.String())
	}
	return bodies
}

func dumpBody(format dumpFormat, chunk []byte) string {
	switch format {
	case dumpCanonical:
		return canonicalDumpBody(chunk)
	case dumpChars:
		var out strings.Builder
		for _, b := range chunk {
			fmt.Fprintf(&out, "%4s", dumpCharName(b))
		}
		return out.String()
	case dumpHexBytes:
		var out strings.Builder
		for _, b := range chunk {
			fmt.Fprintf(&out, " %02x", b)
		}
		return out.String()
	case dumpHexWords:
		// Padded to eight slots, which hexdump does and od does not.
		return dumpWords(chunk, " %04x", 8)
	}
	return dumpWords(chunk, " %06o", 0)
}

// dumpWords reads each pair of bytes little-endian, which is what makes `he`
// print as 6568 and not 6865. A trailing odd byte is the low half of a word whose
// high half is zero.
func dumpWords(chunk []byte, format string, padTo int) string {
	var out strings.Builder
	slots := 0
	for index := 0; index < len(chunk); index += 2 {
		word := uint16(chunk[index])
		if index+1 < len(chunk) {
			word |= uint16(chunk[index+1]) << 8
		}
		fmt.Fprintf(&out, format, word)
		slots++
	}
	// hexdump fills a short final line so the columns of a long dump stay
	// aligned; od leaves it short. Each empty slot is as wide as a filled one.
	for slots < padTo {
		out.WriteString(strings.Repeat(" ", len(fmt.Sprintf(format, 0))))
		slots++
	}
	return out.String()
}

// canonicalDumpBody is `hexdump -C`: sixteen hex bytes split into two groups of
// eight, then the printable text between bars.
func canonicalDumpBody(chunk []byte) string {
	var out strings.Builder
	for index := range 16 {
		if index == 8 {
			out.WriteString(" ")
		}
		if index < len(chunk) {
			fmt.Fprintf(&out, "%02x ", chunk[index])
			continue
		}
		out.WriteString("   ")
	}
	out.WriteString(" |")
	for _, b := range chunk {
		if b >= 0x20 && b < 0x7f {
			out.WriteByte(b)
			continue
		}
		out.WriteByte('.')
	}
	out.WriteString("|")
	return out.String()
}

// dumpCharName is od -c: a mnemonic for the escapes and an octal escape for
// anything else unprintable.
func dumpCharName(b byte) string {
	names := map[byte]string{
		0: `\0`, '\a': `\a`, '\b': `\b`, '\t': `\t`,
		'\n': `\n`, '\v': `\v`, '\f': `\f`, '\r': `\r`,
	}
	if name, found := names[b]; found {
		return name
	}
	if b >= 0x20 && b < 0x7f {
		return string(rune(b))
	}
	return fmt.Sprintf("%03o", b)
}
