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
		if err := addTarOperand(archive, view, operand, r.verbose, stderr); err != nil {
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
	native, err := resolveHostPath(ProcessViewFromContext(ctx), r.file)
	if err != nil {
		return nil, nil, operandFailure(r.file, err)
	}
	file, err := os.Create(native)
	if err != nil {
		return nil, nil, operandFailure(r.file, err)
	}
	return file, func() { file.Close() }, nil
}

// addTarOperand walks one operand into the archive, storing slash-separated
// names so the archive is readable on every platform.
func addTarOperand(archive *tar.Writer, view ProcessView, operand string, verbose bool, stderr io.Writer) error {
	native, err := resolveHostPath(view, operand)
	if err != nil {
		return operandFailure(operand, err)
	}
	return filepath.WalkDir(native, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return operandFailure(operand, walkErr)
		}
		relative, err := filepath.Rel(native, current)
		if err != nil {
			return err
		}
		name := path.Join(filepath.ToSlash(operand), filepath.ToSlash(relative))
		if relative == "." {
			name = filepath.ToSlash(operand)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		// Stored with forward slashes and no drive letter, which is what makes the
		// archive readable by tar on any platform -- and what stops this build
		// writing the very drive-qualified names its own extractor refuses.
		header.Name = name
		if entry.IsDir() {
			header.Name += "/"
		}
		if verbose {
			fmt.Fprintln(stderr, header.Name)
		}
		if err := archive.WriteHeader(header); err != nil {
			return err
		}
		if entry.IsDir() || !info.Mode().IsRegular() {
			return nil
		}
		file, err := os.Open(current)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(archive, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
