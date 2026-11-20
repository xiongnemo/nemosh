package main

import (
	"os"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
	"golang.org/x/term"
)

// noteWindowSize gives a session the terminal's size before a prompt, as COLUMNS and LINES; see
// runtime.NoteWindowSize. Asked of an output, since a console tells its size through the screen
// it draws on and not through its input; with no terminal there is nothing to note.
func (c command) noteWindowSize(rt runtime.Runtime) {
	size := c.windowSize
	if size == nil {
		size = func() (int, int, bool) { return consoleSize(c.stderr, c.stdout) }
	}
	if columns, lines, ok := size(); ok {
		rt.NoteWindowSize(columns, lines)
	}
}

// consoleSize is the size of the first stream that is a terminal.
func consoleSize(streams ...any) (int, int, bool) {
	for _, stream := range streams {
		if file, ok := stream.(*os.File); ok {
			if columns, lines, err := term.GetSize(int(file.Fd())); err == nil && columns > 0 && lines > 0 {
				return columns, lines, true
			}
		}
	}
	return 0, 0, false
}
