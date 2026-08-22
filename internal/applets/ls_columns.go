package applets

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/xiongnemo/nemosh/internal/textgrid"
)

// `ls` wrote one entry per line, always. `-1` was accepted as asking for the format already in
// use, and `-C` was refused by name with a comment saying columns were the thing this could not
// do. It could: the cell measuring the line editor uses to place its cursor was one package
// away, and lives in internal/textgrid now where an applet can reach it.
//
// The rule every ls follows is about the *destination* rather than the options: a terminal gets
// columns, a pipe gets one name per line. That is what lets `ls | wc -l` count files while `ls`
// on screen stays readable, and it is why POSIX specifies the two separately. `-C` asks for
// columns anyway, which is the only way to see the layout in a pipe; `-1` and `-l` defeat them.
//
// Width comes from the terminal where there is one and is 80 otherwise -- measured against
// busybox, whose `-C` into a pipe lays twenty names into seven columns of nine cells.

// lsDefaultWidth is the width assumed when columns were asked for but the destination is not a
// terminal to ask.
const lsDefaultWidth = 80

// writeLsNames writes the short-form listing: a grid where one is wanted, one name per line
// otherwise.
//
// A name arrives painted and has to be measured plain, because a colour escape is bytes that
// occupy no cells. That is what textgrid.Item carries, and it is the same care the completion
// listing takes -- getting it wrong shifts every column after the first coloured name.
func writeLsNames(stdout io.Writer, entries []lsEntry, options lsOptions, columns bool) error {
	if !columns {
		for _, entry := range entries {
			if _, err := fmt.Fprintln(stdout, lsDisplayName(entry, options)); err != nil {
				return err
			}
		}
		return nil
	}
	items := make([]textgrid.Item, len(entries))
	for index, entry := range entries {
		items[index] = textgrid.Item{
			Text:  lsDisplayName(entry, options),
			Cells: textgrid.Cells(lsMeasuredName(entry, options)),
		}
	}
	lines, _ := textgrid.GridOf(items, lsTerminalWidth(stdout, options))
	for _, line := range lines {
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return err
		}
	}
	return nil
}

// lsTerminalWidth is how wide the destination is.
func lsTerminalWidth(stdout io.Writer, options lsOptions) int {
	if options.width > 0 {
		return options.width
	}
	if file := stdoutFile(stdout); file != nil {
		if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 0 {
			return width
		}
	}
	return lsDefaultWidth
}

// lsWantsColumns decides the short form's layout.
func lsWantsColumns(options lsOptions, stdout io.Writer) bool {
	if options.long || options.onePerLine {
		return false
	}
	if options.forceColumns || options.width > 0 {
		return true
	}
	return stdoutIsTerminal(stdout)
}

// terminalSource is what a stream implements when it can name the file it ends at.
//
// The shell hands an applet a descriptor-backed writer, not os.Stdout, so asking
// `stdout.(*os.File)` is always false inside the shell -- which is why `ls` laid out no
// columns on a terminal and `--color=auto` coloured nothing. See fd_stream.go.
type terminalSource interface{ TerminalFile() *os.File }

// stdoutFile is the file a stream ends at, or nil when it is not one.
func stdoutFile(stdout io.Writer) *os.File {
	if file, ok := stdout.(*os.File); ok {
		return file
	}
	if source, ok := stdout.(terminalSource); ok {
		return source.TerminalFile()
	}
	return nil
}

// stdoutIsTerminal reports whether the stream draws on a terminal.
func stdoutIsTerminal(stdout io.Writer) bool {
	file := stdoutFile(stdout)
	return file != nil && term.IsTerminal(int(file.Fd()))
}
