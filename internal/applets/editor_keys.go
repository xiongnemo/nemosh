package applets

import (
	"fmt"
	"io"
	"sort"
	"strings"

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

// editorBinding is one key and what it means, with the label the footer shows.
type editorBinding struct {
	key    tcell.Key
	rune   rune
	label  string
	action editorAction
	// describes is the line `-H` prints. Kept beside the binding so the feature
	// list cannot claim a key that is not bound.
	describes string
}

// nanoBindings are nano's, which is the muscle memory people bring from Linux --
// nano is the default git editor on many distributions.
var nanoBindings = []editorBinding{
	{key: tcell.KeyCtrlO, label: "^O Write Out", action: editorSave, describes: "Write the file out with ^O"},
	{key: tcell.KeyCtrlX, label: "^X Exit", action: editorQuit, describes: "Exit with ^X"},
	{key: tcell.KeyCtrlW, label: "^W Where Is", action: editorSearch, describes: "Search with ^W"},
	{key: tcell.KeyCtrlK, label: "^K Cut", action: editorCutLine, describes: "Cut the current line with ^K"},
	{key: tcell.KeyCtrlU, label: "^U Paste", action: editorPasteLine, describes: "Paste it back with ^U"},
	{key: tcell.KeyCtrlG, label: "^G Help", action: editorHelp, describes: "Show the key list with ^G"},
	{key: tcell.KeyCtrlUnderscore, label: "^_ Go To Line", action: editorGoToLine, describes: "Jump to a line with ^_"},
}

// microBindings are micro's, which match Windows and VS Code conventions and so
// need no learning on this platform.
var microBindings = []editorBinding{
	{key: tcell.KeyCtrlS, label: "^S Save", action: editorSave, describes: "Save with ^S"},
	{key: tcell.KeyCtrlQ, label: "^Q Quit", action: editorQuit, describes: "Quit with ^Q"},
	{key: tcell.KeyCtrlF, label: "^F Find", action: editorSearch, describes: "Find with ^F"},
	{key: tcell.KeyCtrlK, label: "^K Cut Line", action: editorCutLine, describes: "Cut the current line with ^K"},
	{key: tcell.KeyCtrlV, label: "^V Paste", action: editorPasteLine, describes: "Paste with ^V"},
	{key: tcell.KeyCtrlG, label: "^G Help", action: editorHelp, describes: "Show the key list with ^G"},
	{key: tcell.KeyCtrlL, label: "^L Go To", action: editorGoToLine, describes: "Jump to a line with ^L"},
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
func (m editorKeyMap) lookup(event *tcell.EventKey) editorAction {
	for _, binding := range m.bindings {
		if binding.key != 0 && event.Key() == binding.key {
			return binding.action
		}
		if binding.rune != 0 && event.Key() == tcell.KeyRune && event.Rune() == binding.rune {
			return binding.action
		}
	}
	return editorNothing
}

// footer is the two-line legend, laid out in columns the way nano's is.
func (m editorKeyMap) footer(width int) []string {
	if width < 20 {
		width = 20
	}
	columns := max(1, width/16)
	rows := (len(m.bindings) + columns - 1) / columns
	lines := make([]string, rows)
	for index, binding := range m.bindings {
		row := index % rows
		lines[row] += fmt.Sprintf("%-16s", binding.label)
	}
	for index := range lines {
		lines[index] = strings.TrimRight(lines[index], " ")
	}
	return lines
}

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
	for _, line := range absent {
		if _, err := fmt.Fprintf(stdout, "\t%s\n", line); err != nil {
			return err
		}
	}
	return nil
}
