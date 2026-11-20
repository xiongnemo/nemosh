package applets

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/term"
)

// vi is busybox-w32's vi (editors/vi.c, with every FEATURE_VI_* its Windows build turns on),
// with vim's behaviour where the two differ on a key a Debian user's fingers know, and some of
// vim's additions; support-matrix.md has the list. It is its own editor rather than nano's
// buffer under other keys: tview's TextArea has no undo to call, a cursor that sits past the
// last character, and an insert that undoes in pieces.

// viMode is what a key means: a command, text, or a line typed after : or /.
type viMode int

const (
	viNormal viMode = iota
	viInsert
	viReplace
	viCommandLine
)

// viEditor is one vi: the buffer, the cursor, the screen and the mode.
type viEditor struct {
	ctx    context.Context
	view   ProcessView
	screen tcell.Screen
	width  int
	height int
	// ready, when a test gives one, is closed once the screen is first drawn.
	ready chan struct{}

	file  viFile
	lines [][]rune
	// row and col are the cursor, a line and a rune in it; want is the display column j and k
	// keep to, busybox's.
	row, col, want int
	// top and left are the first line shown and the first display column, since a long line
	// is scrolled rather than wrapped, as busybox draws it.
	top, left int
	mode      viMode
	// pending is a command key that waits for the one after it, as Z does, and count the
	// number typed before a command, 0 for none.
	pending rune
	count   int
	// keepWant says the command just run chose want itself, as j, k and $ do.
	keepWant bool
	// lastFind and findTarget are the last f, F, t or T and the key it looked for, which ;
	// and , look for again.
	lastFind, findTarget rune
	// op is an operator waiting for its motion, and opCount the count typed before it; quiet
	// says the operator was x or another of the one-key forms, which busybox does not report.
	op      rune
	opCount int
	quiet   bool
	// failed says the command could not do what it was asked: a motion that went nowhere.
	failed bool
	// registers are a to z and busybox's D; register is the one the command uses, which "x
	// names, appending when it was "X, and prefix says one was just named.
	registers [27]viRegister
	register  int
	appending bool
	prefix    bool
	// marks are a to z, and context where the last jump left from, for '' and ``.
	marks   [26]viMark
	context viMark
	// command is the line typed after : or /, its first rune the prompt.
	command []rune
	// message is shown once on the status line, in reverse when bold, as busybox's.
	message string
	bold    bool
	// modified counts the changes since the buffer was last written, busybox's modified_count.
	modified int
	tabstop  int
	quit     bool
}

func newViApplet() Applet {
	return simpleApplet{name: "vi", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "RH", "")
		if err != nil {
			return err
		}
		if options.has('H') {
			_, err := io.WriteString(stdout, viFeatures)
			return err
		}
		line, operands, err := viStartLine(operands)
		if err != nil {
			return err
		}
		if len(operands) > 1 {
			return fmt.Errorf("one file at a time, for now; there is no :next yet")
		}
		editor := openVi(ctx, operands, options.has('R'))
		editor.goToStartLine(line)
		return runViOnTerminal(ctx, editor, stdin)
	}}
}

// viStartLine reads vim's `+LINE` and `+` before the file: the line to start on, 0 for none
// and -1 for the last. busybox has neither, and read `+10` as a file's name.
func viStartLine(operands []string) (int, []string, error) {
	line := 0
	var rest []string
	for _, operand := range operands {
		if !strings.HasPrefix(operand, "+") || len(rest) > 0 {
			rest = append(rest, operand)
			continue
		}
		if operand == "+" {
			line = -1
			continue
		}
		number, err := strconv.Atoi(operand[1:])
		if err != nil || number < 0 {
			return 0, nil, fmt.Errorf("%s: only +LINE and + are understood", operand)
		}
		line = number
	}
	return line, rest, nil
}

// openVi reads the file, or starts an empty buffer, as busybox does before its first draw.
func openVi(ctx context.Context, operands []string, readOnly bool) *viEditor {
	editor := &viEditor{ctx: ctx, view: ProcessViewFromContext(ctx), tabstop: 8, register: viDefaultRegister}
	name := ""
	if len(operands) > 0 {
		name = operands[0]
	}
	editor.file, editor.lines, editor.message = viLoad(editor.view, name, readOnly)
	editor.bold = editor.message != ""
	editor.goToStartLine(0)
	return editor
}

// goToStartLine puts the cursor on the line it opens on, at its first character that is not a
// blank, as vim does; busybox opens at the line's start.
func (e *viEditor) goToStartLine(line int) {
	switch {
	case line < 0:
		e.row = len(e.lines) - 1
	case line > 0:
		e.row = min(line, len(e.lines)) - 1
	}
	e.col = e.firstNonBlank(e.row)
	e.clampColumn()
	e.want = e.displayColumn(e.line(), e.col)
}

// runViOnTerminal is vi on the console it was started from. Like nano it refuses a standard
// input that is not a terminal, since there is no key to read: `vi f < /dev/null` leases a
// file and would wait for ever.
func runViOnTerminal(ctx context.Context, editor *viEditor, stdin io.Reader) error {
	input, release, ok := leaseTopStdin(ctx, stdin)
	if !ok {
		return fmt.Errorf("standard input is not a terminal, so no key can be read")
	}
	defer release()
	if input == nil || !term.IsTerminal(int(input.Fd())) {
		return fmt.Errorf("standard input is not a terminal, so no key can be read")
	}
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("cannot drive this terminal: %v", err)
	}
	return runVi(ctx, screen, editor)
}
