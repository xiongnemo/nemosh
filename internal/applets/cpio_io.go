package applets

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cpio's two halves that touch the filesystem: extracting a member, and building
// an archive from the names on stdin.
//
// Split from cpio.go for the line ceiling, and the seam is the natural one --
// everything here creates or reads a file, everything there parses or reports.

// extract writes one member, or refuses it and says why.
//
// Every name goes through the shared containment check, the same one tar, unzip
// and httpd use: a cpio archive names its own destinations and an initramfs or an
// RPM payload is not a trusted document.
func (r cpioRequest) extract(reader io.Reader, entry cpioEntry, collisions *archiveCollisions, stderr io.Writer) error {
	safe, err := safeArchivePath(entry.name)
	if err != nil {
		// Skipped with the reason rather than aborting: one hostile entry among
		// honest ones should not cost the honest ones. Same choice as unzip.
		fmt.Fprintf(stderr, "cpio: skipping %v\n", err)
		return r.skipBody(reader, entry)
	}
	if err := collisions.check(safe); err != nil {
		fmt.Fprintf(stderr, "cpio: skipping %v\n", err)
		return r.skipBody(reader, entry)
	}
	native, err := resolveHostPath(r.view, safe)
	if err != nil {
		return operandFailure(safe, err)
	}
	if entry.isDir() {
		if err := os.MkdirAll(native, 0o755); err != nil {
			return err
		}
		return r.finish(reader, entry, native, safe, stderr)
	}
	if entry.isSymlink() {
		// The target is the entry's *data*, so it has to be read either way, and
		// it gets the same check a name does: a link is the second way out of the
		// root. Windows needs a privilege to create one, so the link is written as
		// a file holding the target -- refusing outright would drop content the
		// archive carries, and following it silently would be worse.
		target, err := r.readSymlinkTarget(reader, entry)
		if err != nil {
			return err
		}
		if err := safeLinkTarget(safe, target); err != nil {
			fmt.Fprintf(stderr, "cpio: skipping %v\n", err)
			return nil
		}
		fmt.Fprintf(stderr, "cpio: %s is a symlink; writing its target as a regular file\n", safe)
		if err := r.writeFile(native, safe, strings.NewReader(target), int64(len(target))); err != nil {
			return err
		}
		return r.setTime(entry, native)
	}
	if !entry.isRegular() {
		// A device or a socket node cannot be created here at all, and inventing
		// an empty file in its place would misrepresent the archive.
		fmt.Fprintf(stderr, "cpio: skipping %s: not a file, directory or symlink\n", safe)
		return r.skipBody(reader, entry)
	}
	if !r.overwrite {
		if _, err := os.Stat(native); err == nil {
			fmt.Fprintf(stderr, "cpio: %s exists; use -u to overwrite\n", safe)
			return r.skipBody(reader, entry)
		}
	}
	if err := r.writeFile(native, safe, io.LimitReader(reader, entry.size), entry.size); err != nil {
		return err
	}
	return r.finish(reader, entry, native, safe, stderr)
}

// finish is the tail every kind of entry shares: the data padding, the mtime and
// the -v line.
func (r cpioRequest) finish(reader io.Reader, entry cpioEntry, native, safe string, stderr io.Writer) error {
	if err := skipCpioPadding(reader, entry.size); err != nil {
		return err
	}
	if err := r.setTime(entry, native); err != nil {
		return err
	}
	if r.verbose {
		fmt.Fprintln(stderr, safe)
	}
	return nil
}

func (r cpioRequest) readSymlinkTarget(reader io.Reader, entry cpioEntry) (string, error) {
	target := make([]byte, entry.size)
	if _, err := io.ReadFull(reader, target); err != nil {
		return "", fmt.Errorf("cannot read the link target of %s: %v", entry.name, err)
	}
	if err := skipCpioPadding(reader, entry.size); err != nil {
		return "", err
	}
	return string(target), nil
}

// writeFile creates the destination, making its parents only when -d asked.
//
// Without -d a missing parent is an error, which is both references' behaviour and
// worth keeping: an archive whose entries arrive before their directory is
// unusual enough that silently inventing the tree hides a real problem.
func (r cpioRequest) writeFile(native, safe string, body io.Reader, size int64) error {
	if r.directories {
		if err := os.MkdirAll(filepath.Dir(native), 0o755); err != nil {
			return err
		}
	}
	file, err := os.Create(native)
	if err != nil {
		return operandFailure(safe, err)
	}
	written, copyErr := io.Copy(file, body)
	closeErr := file.Close()
	if copyErr != nil {
		os.Remove(native)
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written != size {
		// The archive is truncated: the header promised more than the stream held.
		os.Remove(native)
		return fmt.Errorf("%s is truncated: %d of %d bytes", safe, written, size)
	}
	return nil
}

// setTime honours -m. Without it the extracted file gets *now*, which is what
// both references do -- the mtime in the archive is information about the original
// and only -m says to carry it over.
func (r cpioRequest) setTime(entry cpioEntry, native string) error {
	if !r.keepTime || entry.mtime == 0 {
		return nil
	}
	when := time.Unix(entry.mtime, 0)
	return os.Chtimes(native, when, when)
}

// createArchive reads names from stdin and writes an archive to stdout.
//
// The names are taken exactly as given: cpio archives a *list*, and expanding or
// descending anything would make it a second, worse tar. `-0` reads NUL-separated
// names, which is how `find -print0` hands over a name with a newline in it.
func (r cpioRequest) createArchive(stdin io.Reader, stdout, stderr io.Writer) error {
	destination := stdout
	var file *os.File
	if r.file != "" {
		native, err := resolveHostPath(r.view, r.file)
		if err != nil {
			return operandFailure(r.file, err)
		}
		if file, err = os.Create(native); err != nil {
			return operandFailure(r.file, err)
		}
		defer file.Close()
		destination = file
	}
	counted := &countingWriter{inner: destination}
	writer := bufio.NewWriter(counted)
	// A serial number per entry rather than the real inode: Windows does not offer
	// one through os.FileInfo, and a header field that is always zero would claim
	// every member is the same file.
	serial := int64(0)
	for _, name := range splitArchiveNames(stdin, r.nulSplit) {
		serial++
		if err := r.addOne(writer, name, serial, stderr); err != nil {
			return err
		}
	}
	if err := writeCpioTrailer(writer); err != nil {
		return err
	}
	// Flushed before the count is read, or it reports the buffer rather than the
	// archive -- which for anything under 4 KB is zero blocks.
	if err := writer.Flush(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(stderr, "%d blocks\n", cpioBlocks(counted.total))
	return err
}

func (r cpioRequest) addOne(writer io.Writer, name string, serial int64, stderr io.Writer) error {
	native, err := resolveHostPath(r.view, name)
	if err != nil {
		return operandFailure(name, err)
	}
	info, err := os.Lstat(native)
	if err != nil {
		return operandFailure(name, err)
	}
	// Stored slash-separated whatever this platform uses, because that is what the
	// format says and what every other reader expects.
	entry := cpioEntry{
		name:  filepath.ToSlash(name),
		ino:   serial,
		nlink: 1,
		mtime: info.ModTime().Unix(),
		mode:  cpioModeOf(info),
	}
	if info.IsDir() {
		if r.verbose {
			fmt.Fprintln(stderr, entry.name)
		}
		return writeCpioEntry(writer, entry, nil)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(native)
		if err != nil {
			return operandFailure(name, err)
		}
		entry.size = int64(len(target))
		if r.verbose {
			fmt.Fprintln(stderr, entry.name)
		}
		return writeCpioEntry(writer, entry, strings.NewReader(target))
	}
	source, err := os.Open(native)
	if err != nil {
		return operandFailure(name, err)
	}
	defer source.Close()
	entry.size = info.Size()
	if r.verbose {
		fmt.Fprintln(stderr, entry.name)
	}
	return writeCpioEntry(writer, entry, source)
}

// cpioModeOf builds the Unix mode a cpio header carries.
//
// Windows has no execute bit and no group or other permissions, so the mode is
// synthesised from what it does have: 0644 for a file, 0755 for a directory, and
// 0444 when the read-only attribute is set. Writing whatever os.FileMode reports
// would put Go's own bit layout into an archive field that means something else.
func cpioModeOf(info os.FileInfo) int64 {
	switch {
	case info.IsDir():
		return 0o040000 | 0o755
	case info.Mode()&os.ModeSymlink != 0:
		return 0o120000 | 0o777
	case info.Mode().Perm()&0o200 == 0:
		return 0o100000 | 0o444
	default:
		return 0o100000 | 0o644
	}
}

// splitArchiveNames reads the name list. Blank lines are dropped, and a trailing
// carriage return with it: a list built on this platform is very likely to have
// CRLF endings, and an entry called "a.txt\r" would be refused as a name that
// does not exist.
func splitArchiveNames(reader io.Reader, nulSeparated bool) []string {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	if nulSeparated {
		// xargs' splitter, not a second one: -0 means the same thing in both
		// places, and `find -print0 | cpio -o0` is the same handshake as
		// `find -print0 | xargs -0`.
		scanner.Split(scanNulSeparated)
	}
	names := make([]string, 0, 16)
	for scanner.Scan() {
		name := strings.TrimRight(scanner.Text(), "\r")
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}
