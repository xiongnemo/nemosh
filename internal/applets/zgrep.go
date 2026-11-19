package applets

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
)

// zgrep, zegrep and zfgrep are grep over what compressed data holds: gzip's zgrep, which Debian
// has with xzgrep beside it, and busybox has not. A rotated log is `syslog.2.gz` beside `syslog`,
// so `zgrep error /var/log/syslog*` has to read both, and does as `gzip -cdf` does: data that
// begins with gzip's, bzip2's or xz's magic is read through its decompressor, zcat's, and any
// other as it is. -r is refused, as gzip's zgrep refuses it: it reads the FILEs it is given.

func newZgrepApplet() Applet  { return zgrepAs("zgrep", nil) }
func newZegrepApplet() Applet { return zgrepAs("zegrep", []string{"-E"}) }
func newZfgrepApplet() Applet { return zgrepAs("zfgrep", []string{"-F"}) }

func zgrepAs(name string, options []string) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		return grepStatus(runGrep(ctx, append(options, args...), true, stdin, stdout, stderr))
	}}
}

var errZgrepRecursive = errors.New("-r is not supported: zgrep reads the FILEs it is given, as gzip's does")

// zgrepLayer is what input holds: decompressed where its first bytes are gzip's, bzip2's or xz's
// magic, and otherwise as it is. The magic is matched further than zcat matches it -- gzip's
// method byte, bzip2's level -- so a text file that happens to begin `BZ` is text. A fault in the
// data names the file it is in: there may be several.
func zgrepLayer(name string, input io.Reader) (io.Reader, error) {
	buffered := bufio.NewReader(input)
	head, _ := buffered.Peek(len(xzMagic))
	compressed := bytes.HasPrefix(head, []byte{0x1f, 0x8b, 0x08}) || bytes.HasPrefix(head, xzMagic) ||
		len(head) >= 4 && bytes.HasPrefix(head, []byte("BZh")) && head[3] >= '1' && head[3] <= '9'
	if !compressed {
		return buffered, nil
	}
	reader, err := decompressor("", buffered)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return zgrepFaults{name: name, reader: reader}, nil
}

// zgrepFaults names the file a decompressor's fault is in.
type zgrepFaults struct {
	name   string
	reader io.Reader
}

func (z zgrepFaults) Read(buffer []byte) (int, error) {
	n, err := z.reader.Read(buffer)
	if err != nil && err != io.EOF {
		err = fmt.Errorf("%s: %w", z.name, err)
	}
	return n, err
}

// grepOpened is what grep reads of an opened input: decompressed first under zgrep, then decoded
// as text, so a BOM inside compressed data is found.
func grepOpened(name string, raw io.Reader, closer io.Closer, decompress bool) (io.ReadCloser, error) {
	reader := raw
	if decompress {
		layered, err := zgrepLayer(name, raw)
		if err != nil {
			return nil, errors.Join(err, closer.Close())
		}
		reader = layered
	}
	return decodedCloser{Reader: decodeTextInput(reader), closer: closer}, nil
}
