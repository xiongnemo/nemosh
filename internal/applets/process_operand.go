package applets

import (
	"context"
	"io"
	"os"
)

// Opening a file operand, where a lone `-` is standard input.
//
// Split from process_view.go for the size ceiling, not because it is a separate
// idea: it is the same seam, and this is the half that knows about operands
// rather than about paths.

// OpenProcessOperand opens one file operand, where a lone `-` names standard
// input.
//
// POSIX gives `-` that meaning for every utility that takes file operands, and
// it is how a script mixes a stream into a list of files:
// `cat header.txt - footer.txt`. Eleven applets here answered
// `No such file or directory` for it instead -- cat, head, tail, wc, grep, sed,
// sort, nl, rev, base64 and md5sum -- while cut, uniq, paste and comm each
// handled it with their own `operand == "-"` check. Four private answers and
// eleven omissions is what one helper is for.
func OpenProcessOperand(ctx context.Context, view ProcessView, path string, stdin io.Reader) (io.ReadCloser, error) {
	if path == "-" {
		return stdinOperand{reader: stdin}, nil
	}
	return OpenProcessInput(ctx, view, path)
}

// stdinOperand is standard input presented as a file operand.
//
// Close is a no-op, because `cat - -` and `cat - f.txt` must not close the
// shell's own stdin when the first operand is finished with.
//
// Read and ReadContext forward rather than being served by io.NopCloser, and
// that is not caution for its own sake: **a wrapper hiding what it wraps has
// been the same bug three times in this package.** descriptorWriter hid
// TerminalFile, synchronizedWriter hid it again, and contextReader hid
// LeaseStdinFile. The shell hands an applet a stdin that implements ReadContext
// so that Ctrl-C interrupts a read; a plain NopCloser would drop that, and
// `cat -` alone would stop being interruptible.
type stdinOperand struct{ reader io.Reader }

func (s stdinOperand) Read(buffer []byte) (int, error) { return s.reader.Read(buffer) }
func (s stdinOperand) Close() error                    { return nil }
func (s stdinOperand) ReadContext(ctx context.Context, buffer []byte) (int, error) {
	return readWithContext(ctx, s.reader, buffer)
}

func openProcessInput(view ProcessView, path string) (io.ReadCloser, error) {
	if opener, ok := view.(processInputView); ok {
		return opener.OpenProcessInput(path)
	}
	native, err := resolveHostPath(view, path)
	if err != nil {
		return nil, err
	}
	return OpenHostInput(native)
}

// processOutputView is what a view implements when it can open a device an applet writes to,
// the mirror of processInputView. The runtime's is runtime.Runtime.OpenProcessOutput.
type processOutputView interface {
	OpenProcessOutput(path string, flag int, perm os.FileMode) (io.WriteCloser, error)
}

// openProcessOutput opens path for an applet to write: a file, or through the view a device,
// /dev/null, /dev/stdout, /dev/stderr, /dev/fd/N and /dev/clipboard, which a redirection takes.
// Each was "not a host path" to `tee`, `dd of=`, `sort -o`, uniq's OUTPUT and sed's w.
func openProcessOutput(view ProcessView, path string, flag int, perm os.FileMode) (io.WriteCloser, error) {
	if opener, ok := view.(processOutputView); ok {
		return opener.OpenProcessOutput(path, flag, perm)
	}
	native, err := resolveHostPath(view, path)
	if err != nil {
		return nil, err
	}
	return os.OpenFile(native, flag, perm)
}
