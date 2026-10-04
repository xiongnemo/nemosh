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
// Width is busybox's: the terminal's, one less, where there is one, and 79 otherwise -- measured,
// sixteen eight-letter names into a pipe take seven columns there, where eighty would fit eight.
// `-w 0` is no limit at all.

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
			if _, err := fmt.Fprintln(stdout, lsEntryPrefix(entry, options)+lsDisplayName(entry, options)); err != nil {
				return err
			}
		}
		return nil
	}
	// busybox's column width: the widest name, without its indicator, and two, and twenty
	// more for -i's column and five for -s's -- which is seven wide, so under -s the widest
	// name meets the next column, as it does in busybox's.
	field := 0
	items := make([]textgrid.Item, len(entries))
	for index, entry := range entries {
		prefix := lsEntryPrefix(entry, options)
		field = max(field, textgrid.Cells(lsNameText(entry.name, options)))
		items[index] = textgrid.Item{
			Text:  prefix + lsDisplayName(entry, options),
			Cells: len(prefix) + textgrid.Cells(lsMeasuredName(entry, options)),
		}
	}
	field += 2
	if options.inode {
		field += 20
	}
	if options.blocks {
		field += 5
	}
	lines, _ := textgrid.GridWith(items, lsTerminalWidth(stdout, options), field, options.across)
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
		if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 1 {
			return width - 1
		}
	}
	return lsDefaultWidth - 1
}

// lsWantsColumns decides the short form's layout.
func lsWantsColumns(options lsOptions, stdout io.Writer) bool {
	if options.long || options.onePerLine {
		return false
	}
	// -w says how wide columns are, not that there are any: busybox's `ls -w 40` into a pipe
	// is one name a line.
	if options.forceColumns || options.across {
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

// TerminalColumns is how many columns w draws in, or 80 when it is no terminal, as busybox's
// get_terminal_width answers: watch's heading is that wide.
func TerminalColumns(w io.Writer) int {
	if file := stdoutFile(w); file != nil {
		if width, _, err := term.GetSize(int(file.Fd())); err == nil && width > 0 {
			return width
		}
	}
	return 80
}
