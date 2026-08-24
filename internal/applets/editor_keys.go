package applets

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// The editor's two key maps, and the feature list generated from them.
//
// One implementation, two names: `nano` gets nano's bindings and `micro` gets
// micro's, chosen by the name the applet was invoked as. That is how busybox
// varies behaviour by argv[0], and it is the honest way to offer both -- calling
// something `nano` and then binding ^S to save would be a name that lies.
//
// Writing it here rather than importing is not a compromise. micro's editing
// core lives entirely under `internal/`, which Go forbids importing across
// modules, and it depends on a *fork* of tcell where this build uses upstream --
// two tcells driving one Windows console is the conflict top_view.go:46 already
// documents. busybox does the same thing with `vi`: its header reads
// "tiny vi.c: A small 'vi' clone", about 3000 lines written from scratch, keeping
// the name and the key language.

// editorAction is what a key does.
type editorAction byte

const (
	editorNothing editorAction = iota
	editorSave
	editorQuit
	editorSearch
	editorCutLine
	editorPasteLine
	editorHelp
	editorGoToLine
)

// editorChord is a modified rune that means a binding: Ctrl and a letter, or Alt
// and one.
//
// Needed because Windows does not deliver a control key as a Key constant the way a
// terminal does. tcell has no VT screen on Windows, so input goes through the console
// API, and for a control character with Ctrl held it adds 0x60 back and posts a *rune
// with ModCtrl* instead (console_win.go:725-736). Which of the two spellings arrives
// therefore depends on what the console reports for the physical key -- and for `^_`
// on a real keyboard, neither did.
type editorChord struct {
	mods tcell.ModMask
	rune rune
}

// editorBinding is one key and what it means, with the label the footer shows.
type editorBinding struct {
	key tcell.Key
	// chords are the modified runes that mean this binding as well as key does.
	// Listing several is deliberate: `^_` has no key of its own on most layouts and
	// is typed as Ctrl and one of several punctuation marks, which is why nano's own
	// help offers `^/` beside it.
	chords []editorChord
	label  string
	action editorAction
	// describes is the line `-H` prints. Kept beside the binding so the feature
	// list cannot claim a key that is not bound.
	describes string
}

// nanoBindings are nano's, which is the muscle memory people bring from Linux --
// nano is the default git editor on many distributions.
var nanoBindings = []editorBinding{
	{key: tcell.KeyCtrlO, chords: ctrl('o'), label: "^O Write Out", action: editorSave, describes: "Write the file out with ^O"},
	{key: tcell.KeyCtrlX, chords: ctrl('x'), label: "^X Exit", action: editorQuit, describes: "Exit with ^X"},
	{key: tcell.KeyCtrlW, chords: ctrl('w'), label: "^W Where Is", action: editorSearch, describes: "Search with ^W"},
	{key: tcell.KeyCtrlK, chords: ctrl('k'), label: "^K Cut", action: editorCutLine, describes: "Cut the current line with ^K"},
	{key: tcell.KeyCtrlU, chords: ctrl('u'), label: "^U Paste", action: editorPasteLine, describes: "Paste it back with ^U"},
	{key: tcell.KeyCtrlG, chords: ctrl('g'), label: "^G Help", action: editorHelp, describes: "Show the key list with ^G"},
	// `^_` did nothing on a real Windows keyboard: the console sends no 0x1F for the
	// chords that are supposed to produce it, so the Key constant never arrived. nano's
	// own help offers `^/` beside `^_` for exactly this reason, and the label leads with
	// the one that works rather than with the one that reads better.
	{
		key:       tcell.KeyCtrlUnderscore,
		chords:    append(ctrl('_', '/', '-'), editorChord{mods: tcell.ModAlt, rune: 'g'}),
		label:     "^/ Go To Line",
		action:    editorGoToLine,
		describes: "Jump to a line with ^/ (also ^_ and M-G)",
	},
}

// microBindings are micro's, which match Windows and VS Code conventions and so
// need no learning on this platform.
var microBindings = []editorBinding{
	{key: tcell.KeyCtrlS, chords: ctrl('s'), label: "^S Save", action: editorSave, describes: "Save with ^S"},
	{key: tcell.KeyCtrlQ, chords: ctrl('q'), label: "^Q Quit", action: editorQuit, describes: "Quit with ^Q"},
	{key: tcell.KeyCtrlF, chords: ctrl('f'), label: "^F Find", action: editorSearch, describes: "Find with ^F"},
	{key: tcell.KeyCtrlK, chords: ctrl('k'), label: "^K Cut Line", action: editorCutLine, describes: "Cut the current line with ^K"},
	{key: tcell.KeyCtrlV, chords: ctrl('v'), label: "^V Paste", action: editorPasteLine, describes: "Paste with ^V"},
	{key: tcell.KeyCtrlG, chords: ctrl('g'), label: "^G Help", action: editorHelp, describes: "Show the key list with ^G"},
	{key: tcell.KeyCtrlL, chords: ctrl('l'), label: "^L Go To", action: editorGoToLine, describes: "Jump to a line with ^L"},
}

// editorKeyMap is one name's bindings.
type editorKeyMap struct {
	name     string
	bindings []editorBinding
}

func editorKeyMapFor(name string) editorKeyMap {
	if name == "micro" {
		return editorKeyMap{name: name, bindings: microBindings}
	}
	return editorKeyMap{name: "nano", bindings: nanoBindings}
}

// lookup finds the action a key press means, if any.
//
// Both spellings are accepted for every binding, because which one a platform sends is
// not something this code can know: the Key constant is what a terminal delivers, and
// the modified rune is what the Windows console does. Accepting both cannot make a
// working key stop working, and it is what makes `^_` reachable at all here.
func (m editorKeyMap) lookup(event *tcell.EventKey) editorAction {
	for _, binding := range m.bindings {
		if binding.key != 0 && event.Key() == binding.key {
			return binding.action
		}
		if event.Key() != tcell.KeyRune {
			continue
		}
		for _, chord := range binding.chords {
			// Shift is ignored, and that is the point rather than laxity: `_` is typed
			// with Shift on this keyboard and `/` is not, so whether Shift is held is a
			// fact about the layout and not about the binding. Every other modifier must
			// match exactly, so Alt-g is not Ctrl-g.
			if event.Modifiers()&^tcell.ModShift != chord.mods {
				continue
			}
			if unicode.ToLower(event.Rune()) == chord.rune {
				return binding.action
			}
		}
	}
	return editorNothing
}

// ctrl and alt build the chord lists, so a binding reads as a row rather than as a
// struct literal per spelling.
func ctrl(runes ...rune) []editorChord { return chords(tcell.ModCtrl, runes) }

func chords(mods tcell.ModMask, runes []rune) []editorChord {
	list := make([]editorChord, 0, len(runes))
	for _, r := range runes {
		list = append(list, editorChord{mods: mods, rune: r})
	}
	return list
}

// footer is the key legend, laid out in columns the way nano's is.
//
// **Compact columns, and as many rows as they need.** Stretching the columns to fill
// the width was tried and looked wrong: with seven bindings on a two-hundred-column
// terminal it left fifty blank characters between each label, which reads as a bug
// rather than as a layout. nano keeps its shortcuts in narrow columns and simply fits
// more of them across a wide window, so this does too -- seven labels are one row at a
// hundred and twelve columns and two rows below that.
//
// The width therefore decides the *height* as well, which is the part the caller has to
// deal with: see editorView.layoutFooter, which resizes the row the legend sits in. The
// original bug was not this function -- it handled width correctly -- but that the
// editor asked it for a layout at a hardcoded 80 exactly once, so a wider terminal kept
// the 80-column answer and left the right-hand side empty.
func (m editorKeyMap) footer(width int) []string {
	if width < footerColumnWidth {
		width = footerColumnWidth
	}
	columns := max(1, width/footerColumnWidth)
	rows := (len(m.bindings) + columns - 1) / columns
	lines := make([]string, rows)
	for index, binding := range m.bindings {
		// Column-major, so a reader looks down a column rather than across a row --
		// which is how nano orders its own list.
		lines[index%rows] += fmt.Sprintf("%-*s", footerColumnWidth, binding.label)
	}
	for index := range lines {
		lines[index] = strings.TrimRight(lines[index], " ")
	}
	return lines
}

// footerColumnWidth is the width of one key label's column. Sixteen because the longest
// label is thirteen characters and two of space either side reads as a column rather
// than as a run-on.
const footerColumnWidth = 16

// writeFeatures is `-H`, in the manner of `busybox vi -H`: the editor states what
// subset it is rather than leaving somebody to discover the gaps.
//
// Generated from the binding table, so it cannot claim a key the editor does not
// bind. That is the property the test checks by walking both.
func (m editorKeyMap) writeFeatures(stdout io.Writer) error {
	lines := []string{
		"Arrow keys, Home, End, PgUp and PgDn to move",
		"Type to insert; Backspace and Delete to remove",
		"Enter to split a line",
	}
	for _, binding := range m.bindings {
		lines = append(lines, binding.describes)
	}
	sort.Strings(lines)
	if _, err := fmt.Fprintf(stdout, "%s implements these features:\n", m.name); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(stdout, "\t%s\n", line); err != nil {
			return err
		}
	}
	// What is absent is named too, because an editor that silently lacks undo is
	// worse than one that says it lacks it. tview's TextArea does provide undo,
	// which is why it is not on this list.
	absent := []string{
		"No multiple buffers",
		"No replace; search only",
		"No mouse",
		"No configuration file",
		// Long lines scroll sideways instead. Not a preference: highlighting needs one
		// screen row to be one buffer line, because tview's line-start table for a
		// wrapped row is unexported and there is no other way to know which line a row
		// is showing. micro does the same.
		"No soft wrap; long lines scroll",
	}
	// And what is present, since it is new and a reader has no other way to find out
	// which languages have rules.
	if _, err := fmt.Fprintf(stdout, "\tSyntax highlighting for %s\n",
		strings.Join(highlightLanguageNames(), ", ")); err != nil {
		return err
	}
	// On by default, which nano is not -- so it is worth saying, and `-l` is accepted
	// so that muscle memory for asking costs nothing.
	if _, err := fmt.Fprintf(stdout, "\tLine numbers in the left margin, always (-l is accepted)\n"); err != nil {
		return err
	}
	for _, line := range absent {
		if _, err := fmt.Fprintf(stdout, "\t%s\n", line); err != nil {
			return err
		}
	}
	return nil
}
