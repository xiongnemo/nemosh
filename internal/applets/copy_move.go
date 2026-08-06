package applets

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// pathOperand pairs a resolved host path with the operand the user typed, so
// the syscall can use the first while the diagnostic names the second.
type pathOperand struct {
	host    string
	operand string
}

func newCpApplet() Applet {
	return simpleApplet{name: "cp", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if len(args) != 2 {
			return ErrExitFalse
		}
		source, dest, err := copyOperands(ctx, args)
		if err != nil {
			return err
		}
		return copyFile(source, dest)
	}}
}

func newMvApplet() Applet {
	return simpleApplet{name: "mv", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if len(args) != 2 {
			return ErrExitFalse
		}
		source, dest, err := copyOperands(ctx, args)
		if err != nil {
			return err
		}
		renameErr := os.Rename(source.host, dest.host)
		if renameErr == nil {
			return nil
		}
		// Only a cross-device rename is worth retrying as copy-then-delete.
		// Anything else is a real failure, and busybox reports it as one
		// instead of falling through to the copy (coreutils/mv.c:137).
		if !isCrossDeviceRename(renameErr) {
			return cannotRename(source.operand, renameErr)
		}
		if err := copyFile(source, dest); err != nil {
			return err
		}
		if err := os.Remove(source.host); err != nil {
			return cannotRemove(source.operand, err)
		}
		return nil
	}}
}

func copyOperands(ctx context.Context, args []string) (pathOperand, pathOperand, error) {
	view := ProcessViewFromContext(ctx)
	sourceHost, err := resolveHostPath(view, args[0])
	if err != nil {
		return pathOperand{}, pathOperand{}, err
	}
	destHost, err := resolveHostPath(view, args[1])
	if err != nil {
		return pathOperand{}, pathOperand{}, err
	}
	source := pathOperand{host: sourceHost, operand: args[0]}
	dest := pathOperand{host: destHost, operand: args[1]}
	return source, copyDestination(source, dest), nil
}

// cp a.txt d writes d/a.txt, so a diagnostic about the destination has to name
// that joined path rather than the bare operand (libbb/copy_file.c:33).
func copyDestination(source, dest pathOperand) pathOperand {
	info, err := os.Stat(dest.host)
	if err != nil || !info.IsDir() {
		return dest
	}
	base := filepath.Base(source.host)
	return pathOperand{
		host:    filepath.Join(dest.host, base),
		operand: filepath.ToSlash(filepath.Join(dest.operand, base)),
	}
}

// The source is stat'd before it is opened only so the two failures can be told
// apart, which is the distinction busybox draws between "can't stat"
// (libbb/copy_file.c:98) and open_or_warn's "can't open".
func copyFile(source, dest pathOperand) error {
	if _, err := os.Stat(source.host); err != nil {
		return cannotStat(source.operand, err)
	}
	reader, err := os.Open(source.host)
	if err != nil {
		return cannotOpen(source.operand, err)
	}
	writer, err := os.Create(dest.host)
	if err != nil {
		if closeErr := reader.Close(); closeErr != nil {
			return closeErr
		}
		return cannotCreate(dest.operand, err)
	}
	_, copyErr := io.Copy(writer, reader)
	readerCloseErr := reader.Close()
	writerCloseErr := writer.Close()
	if copyErr != nil {
		return copyErr
	}
	if readerCloseErr != nil {
		return readerCloseErr
	}
	return writerCloseErr
}
