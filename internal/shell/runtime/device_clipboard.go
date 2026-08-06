package runtime

import (
	"bytes"
	"io"
	"strings"
	"sync"
)

// /dev/clipboard is a Nemosh extension, not a busybox-w32 device: busybox knows
// only stdin, stdout, stderr, null, tty, zero and urandom
// (win32/mingw.c:195-203). Nemosh promises it as text-only, UTF-8 at the
// shell/applet boundary (docs/design/windows-path-model.md:241), so the two
// directions have to agree on line endings as well as encoding.

// clipboardTextForShell converts what Windows hands back into what a script
// expects. Only a CRLF pair collapses; a lone CR stays data, which is the rule
// the rest of the shell follows (docs/design/windows-execution-model.md, "Line
// Endings"). strings.ReplaceAll scans the input, not its own output, so a
// CR immediately before a pair keeps its own byte.
func clipboardTextForShell(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

// clipboardTextForWindows converts what a script wrote into what another
// Windows program expects when it pastes. Collapsing first is what keeps an
// already-CRLF input from doubling.
func clipboardTextForWindows(text string) string {
	return strings.ReplaceAll(clipboardTextForShell(text), "\n", "\r\n")
}

func readClipboardText() (string, error) {
	text, err := readClipboardTextRaw()
	if err != nil {
		return "", err
	}
	return clipboardTextForShell(text), nil
}

func writeClipboardText(text string) error {
	return writeClipboardTextRaw(clipboardTextForWindows(text))
}

// The clipboard holds one value, not a stream, so a read is a snapshot taken
// when the device is opened. A later paste by another program does not reach
// back into a reader a script is still draining.
func openClipboardReader() (io.ReadCloser, error) {
	text, err := readClipboardText()
	if err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(text)), nil
}

// A write is one atomic replacement of that value, so the writer buffers and
// hands the whole text over at Close; setting the clipboard per Write would
// publish half a line. Appending seeds the buffer with what is already there,
// which is the only way `>>` can mean what it says against a slot that cannot
// be seeked.
func openClipboardWriter(appendMode bool) (io.WriteCloser, error) {
	writer := &clipboardWriter{}
	if appendMode {
		existing, err := readClipboardText()
		if err != nil {
			return nil, err
		}
		writer.buffer.WriteString(existing)
	}
	return writer, nil
}

type clipboardWriter struct {
	buffer bytes.Buffer
	once   sync.Once
	err    error
}

func (w *clipboardWriter) Write(p []byte) (int, error) {
	return w.buffer.Write(p)
}

func (w *clipboardWriter) Close() error {
	w.once.Do(func() { w.err = writeClipboardText(w.buffer.String()) })
	return w.err
}
