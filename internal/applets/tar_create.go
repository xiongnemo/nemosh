package applets

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Creating a tar archive. Split from tar.go for the size ceiling; extraction is
// the half that has to distrust its input, and this is the half that produces it.

// tarCreation is an archive being written: the writer, the file it goes to when it is one, and
// whether a name could not be stored.
type tarCreation struct {
	archive *tar.Writer
	stderr  io.Writer
	self    os.FileInfo
	failed  bool
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
	out, release, err := r.createArchiveOutput(ctx, stdout)
	if err != nil {
		return err
	}
	defer release()
	creation := &tarCreation{stderr: stderr}
	if file, ok := out.(interface{ Stat() (os.FileInfo, error) }); ok {
		creation.self, _ = file.Stat()
	}
	stream := out
	var closer io.Closer
	if r.gzip || (r.autoDetect && strings.HasSuffix(strings.ToLower(r.file), ".gz")) {
		writer := gzip.NewWriter(out)
		stream, closer = writer, writer
	}
	creation.archive = tar.NewWriter(stream)
	view := ProcessViewFromContext(ctx)
	for _, operand := range r.operands {
		native, err := resolveHostPath(view, operand)
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
		fmt.Fprintln(stderr, "tar: some names were not archived")
		return ExitStatus(1)
	}
	return nil
}

// passOver says why a name is not stored, and remembers that one was not. The name is bare in
// the saying but for a file that would not open, which is quoted, as busybox shapes the two.
func (c *tarCreation) passOver(err error) {
	fmt.Fprintf(c.stderr, "tar: %v\n", err)
	c.failed = true
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
	if excludedFromArchive(r.selection.reject, name) {
		return nil
	}
	info, err := os.Lstat(native)
	if err == nil && r.dereference {
		info, err = os.Stat(native)
	}
	if err != nil {
		c.passOver(operandFailure(name, err))
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
	// writing the very drive-qualified names its own extractor refuses.
	header.Name = name
	if info.IsDir() {
		header.Name += "/"
	}
	if r.verbose {
		fmt.Fprintln(c.stderr, header.Name)
	}
	if err := c.archive.WriteHeader(header); err != nil {
		return err
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
	for _, entry := range entries {
		inner := path.Join(name, entry.Name())
		if err := r.addTarEntry(c, filepath.Join(native, entry.Name()), inner, append(above, info)); err != nil {
			return err
		}
	}
	return nil
}
