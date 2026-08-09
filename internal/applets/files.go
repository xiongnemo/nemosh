package applets

import (
	"context"
	"errors"
	"io"
)

type catApplet struct{}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	return readWithContext(r.ctx, r.reader, buffer)
}

func (r contextReader) ReadContext(ctx context.Context, buffer []byte) (int, error) {
	return readWithContext(ctx, r.reader, buffer)
}

func readWithContext(ctx context.Context, reader io.Reader, buffer []byte) (int, error) {
	if contextual, ok := reader.(interface {
		ReadContext(context.Context, []byte) (int, error)
	}); ok {
		return contextual.ReadContext(ctx, buffer)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return reader.Read(buffer)
}

func copyWithContext(ctx context.Context, stdout io.Writer, stdin io.Reader) (int64, error) {
	return io.Copy(stdout, contextReader{ctx: ctx, reader: stdin})
}

func newCatApplet() Applet     { return catApplet{} }
func (catApplet) Name() string { return "cat" }
func (catApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
	// An option cat does not implement is refused by name instead of being
	// opened as a file and reported missing.
	given, paths, err := streamOptionsAndOperands("cat", args, "-n")
	if err != nil {
		return err
	}
	// -n numbers every line across all the operands, not per file, which is what
	// makes `cat -n a b` read as one document.
	number := &lineNumberer{on: containsString(given, "-n")}
	if len(paths) == 0 {
		_, err := number.copy(ctx, stdout, stdin)
		return err
	}
	view := ProcessViewFromContext(ctx)
	for _, path := range paths {
		file, err := OpenProcessInput(ctx, view, path)
		if err != nil {
			return cannotOpen(path, err)
		}
		_, copyErr := number.copy(ctx, stdout, file)
		closeErr := file.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return err
		}
	}
	return nil
}
