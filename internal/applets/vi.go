package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

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
	tabstop int
	quit    bool
	// undo and redo are the changes made and undone, seq numbers them, and savedSeq is the one
	// the file holds; changeOpen says the command now running has begun one, undoing that undo
	// or redo is putting one back, and restoringLine that U is. See vi_undo.go.
	undo, redo               []viChange
	seq, savedSeq            int
	changeOpen, undoing      bool
	restoringLine, repeating bool
	// lastPattern is the last search's, lastDirection 1 for / and -1 for ?, searchCompiled it
	// translated and compiled as ignoreCase was then, and searchInput the pattern an operator's
	// / was given. See vi_search.go.
	lastPattern                    string
	lastDirection                  int
	searchCompiled                 *regexp.Regexp
	ignoreCase, searchIgnoringCase bool
	searchInput                    string
	// The options :set sets, beside ignoreCase and tabstop; see vi_set.go. savedCRLF is the
	// file's format when it was read or written, which :set ff changes from.
	autoIndent, binary, expandTab bool
	flash, number, showMatch      bool
	fileFormats                   string
	savedCRLF                     bool
	// shown is lines shown at the foot of the screen until Enter; see vi_message.go.
	shown []string
	// insertStart is where the insert began, which ^U and ^W stop at; replaceStart and replaced
	// are where R began and what each character it typed took the place of, -1 for none;
	// literalNext is a ^V waiting for its key; indentRow and indentLen the line autoindent gave
	// an indent and how long, and matchPause how long showmatch shows a pair. See vi_insert.go.
	insertStart          viPos
	replaceStart         int
	replaced             []rune
	literalNext          bool
	indentRow, indentLen int
	matchPause           time.Duration
	// files are the files given, fileIndex the one being edited, and alternate the one before,
	// #; readOnlyMode is -R, which each file opens with. See vi_files.go.
	files        []string
	fileIndex    int
	alternate    string
	readOnlyMode bool
	// stdin, stdout and stderr are vi's own, which :! gives the command it runs.
	stdin          io.Reader
	stdout, stderr io.Writer
	// lineRow and lineBefore are the line U puts back and what it was.
	lineRow    int
	lineBefore []rune
	// recording is the keys of a command that changes the buffer, kept as lastChange for the
	// dot with the count it had. See vi_dot.go.
	recording, lastChange  []rune
	recordCount, lastCount int
}

func newViApplet() Applet {
	return simpleApplet{name: "vi", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "RHb", "c")
		if err != nil {
			return err
		}
		if options.has('H') {
			_, err := io.WriteString(stdout, viFeatures)
			return err
		}
		startup, files := viStartup(operands)
		editor := openVi(ctx, files, options.has('R'))
		editor.stdin, editor.stdout, editor.stderr = stdin, stdout, stderr
		for _, command := range options.all('c') {
			startup = append(startup, strings.Split(command, "\n")...)
		}
		editor.start(startup, options.has('b'))
		return runViOnTerminal(ctx, editor, stdin)
	}}
}

// viStartup parts the operands into vim's +COMMANDs before the files -- +LINE, + for the last
// line, +/PATTERN, or any ex command -- and the files. busybox has none, and read +10 as a name.
func viStartup(operands []string) ([]string, []string) {
	var startup []string
	for index, operand := range operands {
		if !strings.HasPrefix(operand, "+") {
			return startup, operands[index:]
		}
		switch command := operand[1:]; command {
		case "":
			startup = append(startup, "$")
		default:
			startup = append(startup, command)
		}
	}
	return startup, nil
}

// openVi reads the first file, or starts an empty buffer, as busybox does before its first draw.
func openVi(ctx context.Context, operands []string, readOnly bool) *viEditor {
	editor := &viEditor{ctx: ctx, view: ProcessViewFromContext(ctx), tabstop: 8, register: viDefaultRegister,
		fileFormats: "unix,dos", files: operands, readOnlyMode: readOnly, indentRow: -1,
		matchPause: 400 * time.Millisecond}
	name := ""
	if len(operands) > 0 {
		name = operands[0]
	}
	editor.editFile(name)
	if !editor.bold {
		// Opening says nothing, as busybox's does; a file that cannot be read says why.
		editor.message = ""
	}
	return editor
}

// start runs what busybox runs before the first key -- $EXINIT, or else ~/.exrc, a line a
// command -- then -c's commands and vim's +COMMANDs, a /PATTERN among them searched for. -b,
// busybox's binary, keeps a file's carriage returns in its lines and writes it as it was.
func (e *viEditor) start(commands []string, binary bool) {
	if binary {
		e.binary = true
		if e.file.crlf {
			for index, line := range e.lines {
				e.lines[index] = append(line, '\r')
			}
			e.file.crlf, e.savedCRLF = false, false
		}
	}
	for _, command := range append(e.initCommands(), commands...) {
		if pattern, ok := strings.CutPrefix(command, "/"); ok {
			e.searchFor('/', pattern)
			continue
		}
		e.exCommand(command)
	}
	e.closeChange()
}

// initCommands is $EXINIT, or ~/.exrc when it is not set.
func (e *viEditor) initCommands() []string {
	if text, set := e.view.LookupEnv("EXINIT"); set {
		return strings.Split(text, "\n")
	}
	home, set := e.view.LookupEnv("HOME")
	if !set {
		home, set = e.view.LookupEnv("USERPROFILE")
	}
	if !set {
		return nil
	}
	native, err := resolveHostPath(e.view, home+"/.exrc")
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(native)
	if err != nil {
		return nil
	}
	return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
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
