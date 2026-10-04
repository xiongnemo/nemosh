package applets

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Creating a tar archive. Split from tar.go for the size ceiling; extraction is
// the half that has to distrust its input, and this is the half that produces it.

// tarCreation is an archive being written: the writer, the file it goes to when it is one,
// whether a name could not be stored, and whether a prefix taken off a name has been said.
// names is where -v names each entry: stdout, but stderr when the archive goes there.
type tarCreation struct {
	archive       *tar.Writer
	stderr, names io.Writer
	self          os.FileInfo
	failed, said  bool
}

// create writes the archive. A name that cannot be stored -- one that is not there, a file or a
// directory that cannot be read -- is said and passed over, the rest stored, and the status is
// 1 after a closing word, as busybox goes on; the first such name ended the archive where it
// was, its end never written. The archive is not stored in itself, as busybox passes it over:
// `tar cf a.tar .` failed as the archive grew while it was read.
func (r tarRequest) create(ctx context.Context, stdout, stderr io.Writer) error {
	if len(r.operands) == 0 {
		return fmt.Errorf("no files given to archive")
	}
	// The names are found in -C's directory, as busybox changes to it. It is looked for
	// before the archive is opened, where busybox opens it first and truncates it for a -C
	// that is not there.
	base := ""
	if r.directory != "" {
		root, err := r.extractionRoot(ctx)
		if err != nil {
			return err
		}
		base = root
	}
	// Refused before the archive is opened, so no file is made. A plain tar under a .tbz2 or
	// -j, which is what was written, is a wrong answer that says nothing.
	method := r.creationCompression()
	if method != "" && method != "gzip" && method != "bzip2" {
		return fmt.Errorf("cannot compress with %s: gzip and bzip2 are the compressors here", method)
	}
	out, release, err := r.createArchiveOutput(ctx, stdout)
	if err != nil {
		return err
	}
	defer release()
	creation := &tarCreation{stderr: stderr, names: stdout}
	if r.file == "" || r.file == "-" {
		creation.names = stderr
	}
	if file, ok := out.(interface{ Stat() (os.FileInfo, error) }); ok {
		creation.self, _ = file.Stat()
	}
	stream := out
	var closer io.Closer
	if method != "" {
		writer, err := compressor(out, method, -1)
		if err != nil {
			return err
		}
		stream, closer = writer, writer
	}
	creation.archive = tar.NewWriter(stream)
	view := ProcessViewFromContext(ctx)
	for _, operand := range r.operands {
		native, err := createPath(view, base, operand)
		if err != nil {
			creation.passOver(operandFailure(operand, err))
			continue
		}
		if err := r.addTarEntry(creation, native, filepath.ToSlash(operand), nil); err != nil {
			return err
		}
	}
	if err := creation.archive.Close(); err != nil {
		return err
	}
	if closer != nil {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	if creation.failed {
		fmt.Fprintln(stderr, "tar: error exit delayed from previous errors")
		return ExitStatus(1)
	}
	return nil
}

// createPath is where a name to archive is: under -C's directory when it is relative, and as
// the shell resolves it otherwise. It was always the shell's working directory, and
// `tar -C src -cf a.tar a.txt` did not find a.txt.
func createPath(view ProcessView, base, operand string) (string, error) {
	if base == "" || strings.HasPrefix(filepath.ToSlash(operand), "/") || filepath.VolumeName(operand) != "" {
		return resolveHostPath(view, operand)
	}
	return filepath.Join(base, filepath.FromSlash(operand)), nil
}

// passOver says why a name is not stored, and remembers that one was not. The name is bare in
// the saying but for a file that would not open, which is quoted, as busybox shapes the two.
func (c *tarCreation) passOver(err error) {
	fmt.Fprintf(c.stderr, "tar: %v\n", err)
	c.failed = true
}

// creationCompression is what a new archive is compressed with: -z's gzip, -j's bzip2, or under
// -a what the name ends in, as busybox's tar reads it: gz, bz2, xz or lzma, so a .tgz is gzip as
// a .tar.gz is. -a looked for .gz alone, and wrote a .tgz plain.
func (r tarRequest) creationCompression() string {
	switch {
	case r.gzip:
		return "gzip"
	case r.bzip2:
		return "bzip2"
	case !r.autoDetect:
		return ""
	}
	name := strings.ToLower(r.file)
	for _, rule := range []struct{ suffix, method string }{{"gz", "gzip"}, {"bz2", "bzip2"}, {"xz", "xz"}, {"lzma", "lzma"}} {
		if strings.HasSuffix(name, rule.suffix) {
			return rule.method
		}
	}
	return ""
}

func (r tarRequest) createArchiveOutput(ctx context.Context, stdout io.Writer) (io.Writer, func(), error) {
	if r.file == "" || r.file == "-" {
		return stdout, func() {}, nil
	}
	// A device too: `tar cf /dev/null dir` reads every file and keeps nothing.
	view := ProcessViewFromContext(ctx)
	file, err := openProcessOutput(view, r.file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, createMode(view, 0o666))
	if err != nil {
		return nil, nil, cannotOpen(r.file, err)
	}
	return file, func() { file.Close() }, nil
}

// addTarEntry puts a name into the archive, and what is under it but under --no-recursion. A
// name an exclusion matches is left out, and all under it; -h stores what a symbolic link
// points at, a directory's contents too, and a directory -h comes back to is not gone into
// again. A link is otherwise stored as one, with its target, which it was stored without. The
// error it returns ends the archive; a name it cannot store it passes over.
func (r tarRequest) addTarEntry(c *tarCreation, native, name string, above []os.FileInfo) error {
	info, err := os.Lstat(native)
	if err == nil && r.dereference {
		info, err = os.Stat(native)
	}
	if err != nil {
		c.passOver(operandFailure(name, err))
		return nil
	}
	// Matched once it is known to be there, as busybox stats a name before it asks.
	member := c.memberName(name)
	if excludedFromArchive(r.selection.reject, member) {
		return nil
	}
	if c.self != nil && os.SameFile(c.self, info) {
		fmt.Fprintf(c.stderr, "tar: %s: the archive itself is not stored\n", name)
		return nil
	}
	link := ""
	if info.Mode()&os.ModeSymlink != 0 {
		if link, err = os.Readlink(native); err != nil {
			c.passOver(operandFailure(name, err))
			return nil
		}
	}
	// A file is opened before its header is written, so one that cannot be read is passed over
	// rather than leaving a header with nothing after it.
	var file *os.File
	if info.Mode().IsRegular() {
		if file, err = os.Open(native); err != nil {
			c.passOver(cannotOpen(name, err))
			return nil
		}
		defer file.Close()
	}
	header, err := tar.FileInfoHeader(info, filepath.ToSlash(link))
	if err != nil {
		return err
	}
	// Stored with forward slashes and no drive letter, which is what makes the
	// archive readable by tar on any platform -- and what stops this build
	// writing the very drive-qualified names its own extractor refuses. A name
	// that is all prefix, `/` itself, is not stored; what is under it is.
	header.Name = member
	if info.IsDir() {
		header.Name += "/"
	}
	if member != "" {
		if r.verbose > 0 {
			fmt.Fprintln(c.names, header.Name)
		}
		if err := c.archive.WriteHeader(header); err != nil {
			return err
		}
	}
	if file != nil {
		// The size the header gives, exactly: a file that grew is cut there, and one that
		// shrank ends the archive, whose entry would otherwise run into the next.
		if _, err := io.CopyN(c.archive, file, header.Size); err != nil {
			return operandFailure(name, err)
		}
		return nil
	}
	if !info.IsDir() || r.noRecursion {
		return nil
	}
	for _, ancestor := range above {
		if os.SameFile(ancestor, info) {
			return nil
		}
	}
	entries, err := os.ReadDir(native)
	if err != nil {
		c.passOver(operandFailure(name, err))
		return nil
	}
	// Joined as busybox's concat_path_file joins them, so `.` holds `./a`; they were cleaned,
	// and `.` held `a`.
	for _, entry := range entries {
		inner := strings.TrimSuffix(name, "/") + "/" + entry.Name()
		if err := r.addTarEntry(c, filepath.Join(native, entry.Name()), inner, append(above, info)); err != nil {
			return err
		}
	}
	return nil
}

// memberName is the name an entry is stored by: the name as given, less what busybox's
// skip_unsafe_prefix takes off one -- leading slashes, a leading `../`, all up to the last
// `/../` -- and on Windows a drive before them, which busybox-w32 keeps, though no tar
// extracts it where it says. The first time something is taken off, it is said. Such a name
// was stored whole, and this build's own extraction refused it.
func (c *tarCreation) memberName(name string) string {
	cut := 0
	if runtime.GOOS == "windows" && len(name) >= 2 && name[1] == ':' {
		cut = 2
	}
	cut += unsafePrefix(name[cut:])
	if cut > 0 && !c.said {
		fmt.Fprintf(c.stderr, "tar: removing leading '%s' from member names\n", name[:cut])
		c.said = true
	}
	return name[cut:]
}

// unsafePrefix is how much of a name skip_unsafe_prefix takes off. A name ending in `/..` is
// all prefix; `/..name` is a name.
func unsafePrefix(name string) int {
	at := 0
	for {
		switch {
		case strings.HasPrefix(name[at:], "/"):
			at++
			continue
		case strings.HasPrefix(name[at:], "../"):
			at += 3
			continue
		}
		next := -1
		for search := at; next < 0; {
			found := strings.Index(name[search:], "/..")
			if found < 0 {
				return at
			}
			end := search + found + 3
			switch {
			case end == len(name):
				return end
			case name[end] == '/':
				next = end + 1
			}
			search = end
		}
		at = next
	}
}
