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
	return simpleApplet{name: "cp", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, stderr io.Writer) error {
		options, operands, err := twoOperandsWithOptions(args, "rR")
		if err != nil {
			return err
		}
		if len(operands) > 2 {
			last := len(operands) - 1
			return copyManyOperands(ctx, "cp", operands[:last], operands[last], stderr,
				func(source, dest pathOperand) error {
					return copyOneInto(source, dest, options, stderr)
				})
		}
		source, dest, err := copyOperands(ctx, operands)
		if err != nil {
			return err
		}
		return copyOneInto(source, dest, options, stderr)
	}}
}

// copyOneInto copies one already-resolved source, which is the body both the two-operand form
// and the `source... directory` form need.
//
// A directory needs -r, and saying so is the whole of the difference: without it busybox
// answers `omitting directory` and exits 1 rather than copying something else.
func copyOneInto(source, dest pathOperand, options appletOptions, stderr io.Writer) error {
	if info, statErr := os.Lstat(source.host); statErr == nil && info.IsDir() {
		if !options.has('r') && !options.has('R') {
			return omittingDirectory(source.operand)
		}
		return copyTree(source, dest, stderr)
	}
	return copyFile(source, dest)
}

func newMvApplet() Applet {
	return simpleApplet{name: "mv", runContext: func(ctx context.Context, args []string, _ io.Reader, _ io.Writer, stderr io.Writer) error {
		// -f is accepted and changes nothing, because nothing here prompts:
		// this mv overwrites its destination either way, so -f asks for the
		// behaviour already in force. Scripts carry it constantly, and refusing
		// it made them fail at a request that was already granted.
		_, operands, err := twoOperandsWithOptions(args, "f")
		if err != nil {
			return err
		}
		if len(operands) > 2 {
			last := len(operands) - 1
			return copyManyOperands(ctx, "mv", operands[:last], operands[last], stderr, moveOne)
		}
		source, dest, err := copyOperands(ctx, operands)
		if err != nil {
			return err
		}
		return moveOne(source, dest)
	}}
}

// moveOne renames one already-resolved source, which is the body both the two-operand form
// and the `source... directory` form need.
func moveOne(source, dest pathOperand) error {
	renameErr := os.Rename(source.host, dest.host)
	if renameErr == nil {
		return nil
	}
	// Only a cross-device rename is worth retrying as copy-then-delete. Anything else is a
	// real failure, and busybox reports it as one instead of falling through to the copy
	// (coreutils/mv.c:137).
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
// apart, which is the distinction busybox draws between "cannot stat"
// (libbb/copy_file.c:98) and open_or_warn's "cannot open".
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
