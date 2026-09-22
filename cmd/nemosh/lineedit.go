package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/xiongnemo/nemosh/internal/completionspec"
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
	// kills is what ^K, ^U and ^W took, for ^Y to put back. See lineedit_kill.go.
	kills            killRing
	input            io.Reader
	screen           io.Writer
	workingDirectory string
	// home is the native home directory, so `~/` can be completed. Cached beside the working
	// directory and refreshed with it, because the completion functions hold no Runtime to ask.
	home string
	// devices are the names under /dev, for the same reason and refreshed at the same time.
	devices []string
	buffer  *lineBuffer
	pending []byte
	// afterCarriageReturn remembers that the last key was Enter from a bare CR, so a line
	// feed arriving in the next read is that Enter finishing rather than another one.
	afterCarriageReturn bool
	// afterCookedCommand says a command has just run and read this terminal in cooked
	// mode, so the first key of the next line may be the terminator of a line that
	// command already consumed rather than anything anyone pressed. Set by the session;
	// spent on one line read. See readLine.
	afterCookedCommand bool
	// enterFromPair records that the last Enter was decoded from CRLF rather than from a
	// bare carriage return, which is what tells the two apart.
	enterFromPair bool
	history       []string
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
	// styling decides whether the line is drawn in colour and whether a
	// suggestion is drawn at all. Absent colour turns both off rather than
	// degrading them; see theme.go.
	styling theme
	// suggestion is the text drawn grey after the cursor, held between the draw
	// that computed it and the key that may accept it. Never part of the
	// buffer -- that is what makes it impossible to submit by accident.
	suggestion string
	// commands is what this session can run: builtins and applets, the aliases
	// and functions it has defined, and everything on PATH. It decides both the
	// colour a command word is drawn in and what a suggestion may propose.
	commands *shellCommands
	// hosts is the machines `ssh` could be asked for, read from ~/.ssh/config in
	// the background. A separate index from commands because it answers a
	// different question about a different word, and because it is invalidated
	// differently -- a file's mtime rather than a variable's value.
	hosts *hostIndex
	// specs is what is known about commands this shell does not ship, read from
	// completions/ on first use. Held here rather than looked up globally so a
	// test can hand the editor its own directory.
	specs *completionspec.Registry
	// search is the incremental history search, non-nil only while Ctrl-R has
	// the line. It owns the prompt and most of the keys while it lives; see
	// lineedit_search.go.
	search *historySearch
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
		styling:          newTheme(os.LookupEnv),
		commands:         newShellCommands(newPathIndex()),
		hosts:            newHostIndex(),
		specs:            newSpecRegistry(environmentLookup),
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

	// The terminator of a line the command before this one already consumed. A cooked
	// console read hands back a line ending CRLF, and what the command did not take is
	// still there; decoding it as Enter is an empty command and a spare prompt, which is
	// what ending `wc` with Ctrl-Z used to leave. Only the first key is eligible, only a
	// CRLF pair counts -- in raw mode a pressed Enter is a bare CR -- and the flag is spent
	// whatever the key turns out to be, so it cannot reach a later line.
	leftover := e.afterCookedCommand
	e.afterCookedCommand = false

	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		key, err := e.nextKey()
		if err == nil && leftover {
			leftover = false
			if key.kind == keyEnter && e.enterFromPair {
				continue
			}
		}
		if err != nil {
			// A stream that ends with text already typed submits it, the way a
			// final line without a newline does.
			if errors.Is(err, io.EOF) && !e.buffer.isEmpty() {
				fmt.Fprintln(e.screen)
				return e.buffer.String(), nil
			}
			return "", err
		}
		// The search owns the keyboard while it is up, so the ordinary bindings
		// are not reached: `b` narrows the pattern rather than being inserted.
		// A key the search does not want ends the mode and is then handled
		// normally, in this same pass -- readline lets you land on a line with
		// Ctrl-R and fix it with an arrow, and swallowing that arrow would be a
		// half-in, half-out version of the mode.
		if e.searching() {
			consumed := e.searchKey(key)
			if e.searching() {
				e.drawSearch()
				continue
			}
			e.restorePrompt(prompt)
			if consumed {
				e.redraw(prompt)
				continue
			}
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
			// At the end of the line there is nothing to move onto, so the key
			// is free to mean "take the suggestion" -- which is what fish binds
			// it to, and what makes accepting one keystroke rather than a
			// decision.
			if !e.acceptSuggestion() {
				e.buffer.moveRight()
			}
		case keyHome:
			e.buffer.moveHome()
		case keyEnd:
			if !e.acceptSuggestion() {
				e.buffer.moveEnd()
			}
		case keyClearLine:
			// Backwards to the start of the line, which is readline's
			// unix-line-discard -- so ^U with the cursor in the middle keeps the
			// tail. It cleared the whole line before, a more destructive gesture
			// wearing the same key, and what it removed was gone for good.
			e.kills.kill(e.buffer.killToStart())
		case keyKillToEnd:
			e.kills.kill(e.buffer.killToEnd())
		case keyYank:
			e.buffer.yank(e.kills.yank())
		case keyDeleteWord:
			e.kills.kill(e.buffer.killWord())
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
		case keyHistoryPrefixBackward:
			e.searchHistoryByPrefix(1)
		case keyHistoryPrefixForward:
			e.searchHistoryByPrefix(-1)
		case keyTab:
			e.complete(prompt)
		case keyClearScreen:
			fmt.Fprint(e.screen, "\033[H\033[2J")
			e.resetDrawState()
			fmt.Fprint(e.screen, prompt)
		case keyReverseSearch:
			e.beginHistorySearch()
			e.drawSearch()
			continue
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
		// The line feed of a CRLF that was split across two reads. decodeKey joins the
		// pair when it has both, which is the usual case, but a read boundary can fall
		// between them -- and a bare line feed has to stay an Enter, so the two cannot be
		// told apart without remembering that the last key was a carriage return.
		if e.afterCarriageReturn && len(e.pending) > 0 {
			if e.pending[0] == '\n' {
				e.pending = e.pending[1:]
			}
			e.afterCarriageReturn = false
		}
		if decoded, consumed := decodeKey(e.pending); decoded.kind != keyIncomplete {
			e.afterCarriageReturn = decoded.kind == keyEnter && consumed == 1 && e.pending[0] == '\r'
			e.enterFromPair = decoded.kind == keyEnter && consumed == 2
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

// paths is the editor's snapshot of the shell's path view, as the completion wants it.
func (e *lineEditor) paths() completionPaths {
	return completionPaths{
		workingDirectory: e.workingDirectory,
		home:             e.home,
		devices:          e.devices,
	}
}
