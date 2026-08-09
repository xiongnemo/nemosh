package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// errLineAbandoned is Ctrl-C: the line is discarded and the prompt returns,
// which is not an error the caller should report.
var errLineAbandoned = errors.New("line abandoned")

// lineEditor reads one line a key at a time.
//
// The terminal side is deliberately just an io.Reader and an io.Writer, so the
// whole editor is testable without a terminal. Raw mode is the caller's job;
// see interactive_lineedit.go.
type lineEditor struct {
	input            io.Reader
	screen           io.Writer
	workingDirectory string
	buffer           *lineBuffer
	pending          []byte
	history          []string
	// recall is the index into history being shown, counted from the end.
	// Zero means the line being typed rather than a remembered one.
	recall int
	// drawn is how many columns the last redraw put on screen, so the next one
	// knows how much to erase.
	drawn int
	// drawnRows is how many rows below the prompt's the last redraw reached.
	// Without it a wrapped line cannot be rewritten: `` returns to the start
	// of the current row, not the row the prompt is on.
	drawnRows int
	// width reports the terminal's columns. Injectable so the redraw can be
	// checked without a terminal, and defaulted rather than required so a
	// stream that has no width still edits.
	width func() int
}

// defaultTerminalColumns is used when the terminal will not say. Eighty is the
// conventional answer and is wrong in a harmless direction: a line that does
// not really wrap is redrawn as though it did, which costs a redundant cursor
// move rather than corrupting anything.
const defaultTerminalColumns = 80

func newLineEditor(input io.Reader, screen io.Writer, workingDirectory string) *lineEditor {
	return &lineEditor{
		input:            input,
		screen:           screen,
		workingDirectory: workingDirectory,
		buffer:           newLineBuffer(),
		width:            func() int { return terminalColumns(screen) },
	}
}

// columnsOrDefault keeps the arithmetic safe when the terminal reports nothing
// usable. A zero or negative width would divide by zero; a width of one would
// put every character on its own row.
func (e *lineEditor) columnsOrDefault() int {
	if e.width == nil {
		return defaultTerminalColumns
	}
	if columns := e.width(); columns > 1 {
		return columns
	}
	return defaultTerminalColumns
}

// remember adds a line to the history. A blank line and a repeat of the
// previous entry are skipped, which is what every shell does and what keeps the
// arrows worth pressing.
func (e *lineEditor) remember(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	if len(e.history) > 0 && e.history[len(e.history)-1] == line {
		return
	}
	e.history = append(e.history, line)
}

func (e *lineEditor) entries() []string { return e.history }

// resetDrawState declares that nothing of the previous drawing is on screen any
// more. The two counters have to move together: keeping only the column count
// would leave the next redraw climbing rows that are no longer its own.
func (e *lineEditor) resetDrawState() {
	e.drawn = 0
	e.drawnRows = 0
}

// readLine draws prompt and returns when the user submits a line. io.EOF means
// end of input -- Ctrl-D on an empty line, Ctrl-Z, or the stream running out.
func (e *lineEditor) readLine(ctx context.Context, prompt string) (string, error) {
	e.buffer = newLineBuffer()
	e.recall = 0
	e.resetDrawState()
	fmt.Fprint(e.screen, prompt)

	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		key, err := e.nextKey()
		if err != nil {
			// A stream that ends with text already typed submits it, the way a
			// final line without a newline does.
			if errors.Is(err, io.EOF) && !e.buffer.isEmpty() {
				fmt.Fprintln(e.screen)
				return e.buffer.String(), nil
			}
			return "", err
		}
		switch key.kind {
		case keyEnter:
			fmt.Fprintln(e.screen)
			return e.buffer.String(), nil
		case keyEndOfInput:
			// Only on an empty line. With text in the buffer this is a forward
			// delete, because ending input there would throw the line away.
			if e.buffer.isEmpty() {
				return "", io.EOF
			}
			e.buffer.deleteForward()
		case keyInterrupt:
			fmt.Fprintln(e.screen)
			return "", errLineAbandoned
		case keyRune:
			e.buffer.insert(key.value)
		case keyBackspace:
			e.buffer.backspace()
		case keyDelete:
			e.buffer.deleteForward()
		case keyLeft:
			e.buffer.moveLeft()
		case keyRight:
			e.buffer.moveRight()
		case keyHome:
			e.buffer.moveHome()
		case keyEnd:
			e.buffer.moveEnd()
		case keyClearLine:
			e.buffer.replace("")
		case keyDeleteWord:
			e.buffer.deleteWord()
		case keyDeleteWordForward:
			e.buffer.deleteWordForward()
		case keyWordLeft:
			e.buffer.moveWordLeft()
		case keyWordRight:
			e.buffer.moveWordRight()
		case keyUp:
			e.recallHistory(1)
		case keyDown:
			e.recallHistory(-1)
		case keyTab:
			e.complete(prompt)
		case keyClearScreen:
			fmt.Fprint(e.screen, "\033[H\033[2J")
			e.resetDrawState()
			fmt.Fprint(e.screen, prompt)
		}
		e.redraw(prompt)
	}
}

// recallHistory walks back through what was typed before. Direction is +1 for
// older and -1 for newer; walking past the newest returns the empty line.
func (e *lineEditor) recallHistory(direction int) {
	target := e.recall + direction
	if target < 0 || target > len(e.history) {
		return
	}
	e.recall = target
	if target == 0 {
		e.buffer.replace("")
		return
	}
	e.buffer.replace(e.history[len(e.history)-target])
}

// nextKey decodes one key, reading more bytes when the buffer holds only part
// of a sequence.
func (e *lineEditor) nextKey() (key, error) {
	for {
		if decoded, consumed := decodeKey(e.pending); decoded.kind != keyIncomplete {
			e.pending = e.pending[consumed:]
			return decoded, nil
		}
		chunk := make([]byte, 64)
		count, err := e.input.Read(chunk)
		if count > 0 {
			e.pending = append(e.pending, chunk[:count]...)
			continue
		}
		if err == nil {
			err = io.EOF
		}
		return key{}, err
	}
}
