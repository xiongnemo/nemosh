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

func (r tarRequest) create(ctx context.Context, stdout, stderr io.Writer) error {
	if len(r.operands) == 0 {
		return fmt.Errorf("no files given to archive")
	}
	out, release, err := r.createArchiveOutput(ctx, stdout)
	if err != nil {
		return err
	}
	defer release()
	stream := out
	var closer io.Closer
	if r.gzip || (r.autoDetect && strings.HasSuffix(strings.ToLower(r.file), ".gz")) {
		writer := gzip.NewWriter(out)
		stream, closer = writer, writer
	}
	archive := tar.NewWriter(stream)
	view := ProcessViewFromContext(ctx)
	for _, operand := range r.operands {
		native, err := resolveHostPath(view, operand)
		if err != nil {
			return operandFailure(operand, err)
		}
		if err := r.addTarEntry(archive, native, filepath.ToSlash(operand), nil, stderr); err != nil {
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	if closer != nil {
		return closer.Close()
	}
	return nil
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
// again. A link is otherwise stored as one, with its target, which it was stored without.
func (r tarRequest) addTarEntry(archive *tar.Writer, native, name string, above []os.FileInfo, stderr io.Writer) error {
	if excludedFromArchive(r.selection.reject, name) {
		return nil
	}
	info, err := os.Lstat(native)
	if err == nil && r.dereference {
		info, err = os.Stat(native)
	}
	if err != nil {
		return operandFailure(name, err)
	}
	link := ""
	if info.Mode()&os.ModeSymlink != 0 {
		if link, err = os.Readlink(native); err != nil {
			return operandFailure(name, err)
		}
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
		fmt.Fprintln(stderr, header.Name)
	}
	if err := archive.WriteHeader(header); err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		return copyIntoArchive(archive, native)
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
		return operandFailure(name, err)
	}
	for _, entry := range entries {
		inner := path.Join(name, entry.Name())
		if err := r.addTarEntry(archive, filepath.Join(native, entry.Name()), inner, append(above, info), stderr); err != nil {
			return err
		}
	}
	return nil
}

// copyIntoArchive is a file's contents, after its header.
func copyIntoArchive(archive *tar.Writer, native string) error {
	file, err := os.Open(native)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(archive, file)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
