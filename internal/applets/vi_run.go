package applets

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// runVi draws and reads keys until the editor quits. The screen is handed in, so a test can
// give it a simulation screen.
func runVi(ctx context.Context, screen tcell.Screen, e *viEditor) error {
	if err := screen.Init(); err != nil {
		return fmt.Errorf("cannot drive this terminal: %v", err)
	}
	defer screen.Fini()
	e.screen = screen
	e.width, e.height = screen.Size()
	// A cancelled context has to interrupt PollEvent, which only an event does, until vi
	// leaves; less_run.go has why the goroutine waits on both.
	left := make(chan struct{})
	defer close(left)
	go func() {
		select {
		case <-ctx.Done():
			screen.PostEvent(tcell.NewEventInterrupt(nil))
		case <-left:
		}
	}()
	for !e.quit {
		// Drawn when no key is waiting. A paste is a key a character, and a frame for each
		// took 9 seconds for 4,580 of them in conhost, measured 2026-11-26; the keys already
		// queued are handled first and the frame after them shows what theirs would have
		// ended on. vi reads tcell's queue itself, so what is waiting there is all there is.
		if !screen.HasPendingEvent() {
			e.draw()
			if e.ready != nil {
				close(e.ready)
				e.ready = nil
			}
		}
		event := screen.PollEvent()
		if event == nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		e.handle(event)
	}
	return nil
}

func (e *viEditor) handle(event tcell.Event) {
	switch event := event.(type) {
	case *tcell.EventResize:
		e.width, e.height = e.screen.Size()
		e.screen.Sync()
	case *tcell.EventKey:
		for _, key := range viKeysOf(event) {
			e.key(key)
		}
	}
}

// key is one key, read in the mode the editor is in, and kept for the dot when it is part of a
// command that changes the buffer.
func (e *viEditor) key(key rune) {
	if e.dismissKey(key) {
		return
	}
	if e.confirming != nil {
		e.confirmKey(key)
		return
	}
	if key == '.' && e.mode == viNormal && e.pending == 0 && e.op == 0 {
		e.repeatChange()
		e.count = 0
		return
	}
	e.startRecording(key)
	switch e.mode {
	case viInsert, viReplace:
		e.insertKey(key)
	case viCommandLine:
		e.commandLineKey(key)
	default:
		e.normalKey(key)
	}
	e.finishRecording()
}

// say puts a message on the status line, bold for an error, as busybox's status_line and
// status_line_bold.
func (e *viEditor) say(bold bool, format string, args ...any) {
	e.message, e.bold = fmt.Sprintf(format, args...), bold
}

// beep is busybox's indicate_error, for a key that does nothing where it is typed; a command it
// is part of has failed, which an operator asks.
func (e *viEditor) beep() {
	e.failed = true
	if e.screen != nil {
		_ = e.screen.Beep()
	}
}

// line is the cursor's line.
func (e *viEditor) line() []rune { return e.lines[e.row] }

// clampColumn keeps the cursor on a character, or at the end of the line in insert mode,
// where it may sit after the last one.
func (e *viEditor) clampColumn() {
	last := len(e.line())
	if e.mode == viNormal && last > 0 {
		last--
	}
	e.col = max(0, min(e.col, last))
}
