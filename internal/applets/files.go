package applets

import (
	"context"
	"errors"
	"io"
	"os"
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

// LeaseStdinFile passes the request through to whatever is underneath.
//
// Every applet's stdin arrives wrapped in one of these -- the registry does it, so a long read can
// be cancelled -- and a wrapper that forwards only Read hides everything else the reader could do.
// That is what stopped `top` from ever drawing: it asks for the console so tcell can read keys,
// the request reached this wrapper, and the wrapper did not know how to pass it on. The applet
// then reported, correctly and uselessly, that standard input was not a terminal.
//
// Third time this shape has appeared. descriptorWriter needed TerminalFile for the same reason,
// and then synchronizedWriter needed to forward it, which is why terminalFileOf walks a chain
// rather than checking one hop. A wrapper has to forward what it does not implement.
func (r contextReader) LeaseStdinFile(ctx context.Context) (*os.File, func(), bool) {
	if file, ok := r.reader.(*os.File); ok {
		return file, func() {}, true
	}
	leaser, ok := r.reader.(interface {
		LeaseStdinFile(context.Context) (*os.File, func(), bool)
	})
	if !ok {
		return nil, func() {}, false
	}
	return leaser.LeaseStdinFile(ctx)
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
		// A lone `-` is the stdin, which is how `cat header.txt - footer.txt`
		// mixes a stream into a list of files. See OpenProcessOperand.
		file, err := OpenProcessOperand(ctx, view, path, stdin)
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
