package applets

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/term"
)

// nano and micro: one editor, two names, the key map chosen by the name.
//
// The interactive path follows `top` exactly -- lease the console, build a
// tcell screen, fall back with a reason when there is no terminal -- because that
// is the pattern already proven here and the lease is what stops the shell's own
// reader thread stealing key presses (top_view.go:46).
//
// The text buffer is tview's TextArea, which already handles a cursor, selection,
// double-width characters and undo. Writing a buffer from scratch when a tested
// one is a dependency away would be the wrong kind of thrift.

func newNanoApplet() Applet { return newEditorApplet("nano") }

func newMicroApplet() Applet { return newEditorApplet("micro") }

func newEditorApplet(name string) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		// `l` is nano's line-number flag. It is accepted and does nothing, because the
		// numbers are already on -- refusing a flag whose effect is the current state
		// would be pedantry at the user's expense.
		options, operands, err := parseAppletOptions(args, "HRl", "")
		if err != nil {
			return err
		}
		keys := editorKeyMapFor(name)
		if options.has('H') {
			// In the manner of `busybox vi -H`: say what subset this is.
			return keys.writeFeatures(stdout)
		}
		if len(operands) > 1 {
			return fmt.Errorf("only one file at a time; there are no multiple buffers")
		}
		session, err := openEditorSession(ctx, name, operands, options.has('R'))
		if err != nil {
			return err
		}
		return runEditor(ctx, session, keys, stdin, stderr)
	}}
}

// editorSession is the file being edited.
type editorSession struct {
	name     string
	path     string
	native   string
	text     string
	readOnly bool
	// isNew records that the file did not exist, so the status line can say so
	// rather than showing an empty buffer with no explanation.
	isNew bool
}

func openEditorSession(ctx context.Context, applet string, operands []string, readOnly bool) (*editorSession, error) {
	session := &editorSession{name: applet, readOnly: readOnly}
	if len(operands) == 0 {
		return session, nil
	}
	session.path = operands[0]
	native, err := resolveHostPath(ProcessViewFromContext(ctx), session.path)
	if err != nil {
		return nil, operandFailure(session.path, err)
	}
	session.native = native
	data, err := os.ReadFile(native)
	switch {
	case os.IsNotExist(err):
		// A name that does not exist yet is the normal way to start a new file,
		// so it opens empty rather than failing.
		session.isNew = true
	case err != nil:
		return nil, operandFailure(session.path, err)
	default:
		session.text = string(data)
	}
	return session, nil
}

// save writes the buffer back.
//
// Bytes are written as they are: this editor does not decode, so it cannot
// re-encode, and the file keeps whatever encoding it arrived in. That is the same
// rule `sed -i` follows and the reason neither needs the encoding policy `iconv`
// settles.
func (s *editorSession) save(text string) error {
	if s.readOnly {
		return fmt.Errorf("the file is open read-only")
	}
	if s.native == "" {
		return fmt.Errorf("no file name; start it with a name to write to")
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(s.native); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(s.native, []byte(text), mode); err != nil {
		return err
	}
	s.isNew = false
	return nil
}

// runEditor draws until the user leaves.
func runEditor(ctx context.Context, session *editorSession, keys editorKeyMap, stdin io.Reader, stderr io.Writer) error {
	input, release, ok := leaseTopStdin(ctx, stdin)
	if !ok {
		// No terminal to read keys from. An editor has no useful non-interactive
		// form -- unlike `top`, which can print one sample -- so this refuses and
		// says why rather than appearing to work.
		return fmt.Errorf("standard input is not a terminal, so no key can be read")
	}
	defer release()
	// Leasing is not enough: `nano file < /dev/null` leases successfully, because
	// /dev/null *is* a file, and then the editor draws itself and waits for keys
	// that will never arrive. Measured -- it hung. The question is whether stdin
	// is a terminal, not whether it is a file.
	if input == nil || !term.IsTerminal(int(input.Fd())) {
		return fmt.Errorf("standard input is not a terminal, so no key can be read")
	}
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("cannot drive this terminal: %v", err)
	}
	return runEditorApplication(ctx, session, keys, screen, nil)
}

// runEditorApplication is the event loop, with the screen handed in so a test can
// pass a simulation screen.
//
// ready, when given, receives the application once it is built -- the same device
// top's headless test uses, and for the same reason: the screen may only be read
// from tview's own goroutine.
func runEditorApplication(ctx context.Context, session *editorSession, keys editorKeyMap,
	screen tcell.Screen, ready chan<- *tview.Application) error {
	application := tview.NewApplication().SetScreen(screen)
	view := newEditorView(session, keys, application)
	application.SetRoot(view.layout, true).SetFocus(view.area)
	go func() {
		<-ctx.Done()
		application.Stop()
	}()
	if ready != nil {
		ready <- application
	}
	return application.Run()
}
