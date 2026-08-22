package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// cpio: the archive format that carries a *file list* rather than a directory
// walk, which is the whole reason it still exists.
//
//	find . -name '*.go' | cpio -o -H newc > src.cpio
//
// tar takes the paths it is given and descends them; cpio takes exactly the names
// on its stdin and nothing else. That makes it the pair to `find`, which this
// build has, and it is how initramfs images and RPM payloads are built.
//
// The three modes are exclusive and one is required, which is busybox's shape:
// -t lists, -i extracts, -o creates.

func newCpioApplet() Applet {
	return simpleApplet{name: "cpio", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(args, "tiodmvu0", "FH")
		if err != nil {
			return err
		}
		modes := 0
		for _, letter := range "tio" {
			if options.has(byte(letter)) {
				modes++
			}
		}
		if modes != 1 {
			return fmt.Errorf("exactly one of -t, -i or -o is required")
		}
		if options.has('H') && options.value('H') != "newc" {
			return fmt.Errorf("only -H newc is written; %q is not a format this build produces", options.value('H'))
		}
		request := cpioRequest{
			list:        options.has('t'),
			create:      options.has('o'),
			verbose:     options.has('v'),
			directories: options.has('d'),
			keepTime:    options.has('m'),
			overwrite:   options.has('u'),
			nulSplit:    options.has('0'),
			file:        options.value('F'),
			wanted:      operands,
			view:        ProcessViewFromContext(ctx),
		}
		return request.run(stdin, stdout, stderr)
	}}
}

type cpioRequest struct {
	list        bool
	create      bool
	verbose     bool
	directories bool
	keepTime    bool
	overwrite   bool
	nulSplit    bool
	file        string
	wanted      []string
	view        ProcessView
}

func (r cpioRequest) run(stdin io.Reader, stdout, stderr io.Writer) error {
	if r.create {
		return r.createArchive(stdin, stdout, stderr)
	}
	source := stdin
	if r.file != "" {
		native, err := resolveHostPath(r.view, r.file)
		if err != nil {
			return operandFailure(r.file, err)
		}
		file, err := os.Open(native)
		if err != nil {
			return operandFailure(r.file, err)
		}
		defer file.Close()
		source = file
	}
	counted := &countingReader{inner: bufio.NewReader(source)}
	if err := r.readArchive(counted, stdout, stderr); err != nil {
		return err
	}
	// Both references end with this, on stderr so a pipe is unaffected. It is the
	// only way to know an archive was read whole when the members were skipped.
	_, err := fmt.Fprintf(stderr, "%d blocks\n", cpioBlocks(counted.total))
	return err
}

// readArchive walks the members once, which is all the format allows: there is no
// index, so listing and extracting are the same pass with a different body.
func (r cpioRequest) readArchive(reader io.Reader, stdout, stderr io.Writer) error {
	collisions := newArchiveCollisions()
	for {
		entry, err := readCpioHeader(reader)
		if err != nil {
			return err
		}
		if entry == nil {
			return nil
		}
		if !r.selects(entry.name) {
			if err := r.skipBody(reader, *entry); err != nil {
				return err
			}
			continue
		}
		if r.list {
			// Names are listed exactly as the archive holds them, unchecked,
			// because listing is how somebody inspects an archive they do not
			// trust. Refusing here would hide the entry they are looking for.
			if err := r.writeListing(*entry, stdout); err != nil {
				return err
			}
			if err := r.skipBody(reader, *entry); err != nil {
				return err
			}
			continue
		}
		if err := r.extract(reader, *entry, collisions, stderr); err != nil {
			return err
		}
	}
}

// skipBody advances past an entry's data and its padding, which has to happen for
// every entry whether or not it was wanted -- the reader may be a pipe, so there
// is nothing to seek.
func (r cpioRequest) skipBody(reader io.Reader, entry cpioEntry) error {
	if entry.size > 0 {
		if _, err := io.CopyN(io.Discard, reader, entry.size); err != nil {
			return fmt.Errorf("cannot skip %s: %v", entry.name, err)
		}
	}
	return skipCpioPadding(reader, entry.size)
}

func (r cpioRequest) selects(name string) bool {
	if len(r.wanted) == 0 {
		return true
	}
	for _, pattern := range r.wanted {
		if pattern == name {
			return true
		}
		if matched, err := filepath.Match(pattern, name); err == nil && matched {
			return true
		}
	}
	return false
}

func (r cpioRequest) writeListing(entry cpioEntry, stdout io.Writer) error {
	if !r.verbose {
		_, err := fmt.Fprintln(stdout, entry.name)
		return err
	}
	// busybox's columns exactly, measured: mode, uid/gid, a nine-wide size, then a
	// full timestamp. GNU cpio prints an `ls -l`-style abbreviated date here
	// instead; busybox is the primary reference, and its version does not turn
	// ambiguous six months out.
	when := time.Unix(entry.mtime, 0).Format("2006-01-02 15:04:05")
	_, err := fmt.Fprintf(stdout, "%s %d/%d %9d %s %s\n",
		modeString(entry), entry.uid, entry.gid, entry.size, when, entry.name)
	return err
}

// modeString is the `ls -l` first column, which is what cpio -tv prints. Only the
// three types this build creates are distinguished; anything else reads as `?`
// rather than being claimed as a regular file.
func modeString(entry cpioEntry) string {
	kind := "?"
	switch {
	case entry.isDir():
		kind = "d"
	case entry.isSymlink():
		kind = "l"
	case entry.isRegular():
		kind = "-"
	}
	permissions := ""
	for shift := 6; shift >= 0; shift -= 3 {
		bits := entry.mode >> shift
		for index, letter := range "rwx" {
			if bits&(1<<(2-index)) != 0 {
				permissions += string(letter)
				continue
			}
			permissions += "-"
		}
	}
	return kind + permissions
}
