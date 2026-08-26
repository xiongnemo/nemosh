package applets

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ^G, and why it is a panel.
//
// It used to write the key list onto the message row -- directly above the legend, which
// already shows the key list. So "help" drew a second row of key names under the first,
// which reads as a rendering fault rather than as help. `top` made the same mistake and
// c5a1a44 fixed it the same way: the keys are already on screen, so what a legend owes
// the reader is the things that are *not*.
//
// It also carries a key reader, which is not a debug affordance but the answer to a
// question this editor cannot otherwise answer. `^_` was unreachable on Windows and two
// attempts to guess its spelling both failed, because what a console sends for a chord
// is not knowable from here -- see editor_keys.go. A terminal that can be *asked* what
// it sends turns that from a guess into a measurement, and anyone rebinding a key on a
// terminal this code has never seen needs the same thing.

// showHelp puts the legend over the buffer until Escape or the help key dismisses it.
func (v *editorView) showHelp() {
	if v.application == nil {
		// No application to swap the root of -- the headless tests build a view without
		// one. The message row is the honest fallback rather than a panic.
		v.setMessage("%s", strings.Join(v.keys.helpLines(), "  "))
		return
	}
	panel := tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(true)
	panel.SetText(v.helpPanel(""))
	panel.SetBorder(true).SetTitle(fmt.Sprintf(" nemosh %s -- keys, and what this terminal sends ", v.keys.name))
	panel.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd:
			// The panel can be longer than the terminal, so movement has to reach it.
			return event
		case tcell.KeyEscape:
			v.hideHelp()
			return nil
		}
		if v.keys.lookup(event) == editorHelp {
			v.hideHelp()
			return nil
		}
		// Every other key is named rather than acted on. That is the reader: press the
		// key that does not work and the panel says what arrived, if anything.
		panel.SetText(v.helpPanel(describeKeyEvent(event)))
		return nil
	})
	v.application.SetRoot(panel, true)
}

// hideHelp puts the buffer back.
func (v *editorView) hideHelp() {
	v.application.SetRoot(v.layout, true)
	v.application.SetFocus(v.area)
}

// helpPanel is the legend, with the last key read if there is one.
func (v *editorView) helpPanel(lastKey string) string {
	var out strings.Builder
	out.WriteString("[aqua]KEYS[white]\n")
	for _, binding := range v.keys.bindings {
		fmt.Fprintf(&out, "  [yellow]%-16s[white] %s\n", binding.label, binding.describes)
	}
	fmt.Fprintf(&out, "  [yellow]%-16s[white] %s\n", "^Z  ^Y", "undo and redo, which the text area provides")
	fmt.Fprintf(&out, "  [yellow]%-16s[white] %s\n", "arrows Home End", "move; PgUp and PgDn by a screen")

	out.WriteString("\n[aqua]THIS TERMINAL[white]\n")
	out.WriteString("  Press any key and it is named here rather than acted on, so a binding that\n")
	out.WriteString("  does nothing can be told apart from one the terminal never delivered.\n")
	out.WriteString("  [gray]Escape or the help key closes this panel.[white]\n")
	if lastKey == "" {
		out.WriteString("\n  [gray]nothing read yet[white]\n")
	} else {
		fmt.Fprintf(&out, "\n  [green]%s[white]\n", lastKey)
	}

	out.WriteString("\n[aqua]SYNTAX HIGHLIGHTING[white]\n")
	fmt.Fprintf(&out, "  %s\n", strings.Join(highlightLanguageNames(), ", "))
	fmt.Fprintf(&out, "  [gray]this buffer: %s[white]\n", v.syntaxName())

	out.WriteString("\n[aqua]WHAT IS NOT HERE, AND WHY[white]\n")
	for _, absent := range editorAbsent {
		fmt.Fprintf(&out, "  [yellow]%-22s[white] %s\n", absent.what, absent.why)
	}
	return out.String()
}

func (v *editorView) syntaxName() string {
	if v.area.syntax == nil {
		return "none; the file name matched no language, so it is shown as plain text"
	}
	return v.area.syntax.name
}

// describeKeyEvent names a key press in the terms this code matches on, so that what the
// panel prints can be turned into a binding without another round of guessing.
//
// tcell's own Name() is first because it is the familiar spelling, but the parts matter
// more: on Windows a Ctrl'd letter arrives as a *rune* with ModCtrl rather than as a Key
// constant, and those two look identical in a Name().
func describeKeyEvent(event *tcell.EventKey) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("name %q", event.Name()))
	if event.Key() == tcell.KeyRune {
		parts = append(parts, fmt.Sprintf("rune %q (U+%04X)", event.Rune(), event.Rune()))
	} else {
		parts = append(parts, fmt.Sprintf("key constant %d", int(event.Key())))
	}
	mods := []string{}
	for _, mod := range []struct {
		mask tcell.ModMask
		name string
	}{
		{tcell.ModCtrl, "Ctrl"}, {tcell.ModAlt, "Alt"},
		{tcell.ModShift, "Shift"}, {tcell.ModMeta, "Meta"},
	} {
		if event.Modifiers()&mod.mask != 0 {
			mods = append(mods, mod.name)
		}
	}
	if len(mods) == 0 {
		mods = append(mods, "none")
	}
	parts = append(parts, "modifiers "+strings.Join(mods, "+"))
	return strings.Join(parts, ", ")
}

// editorAbsent is what somebody coming from the real editor will look for and not find,
// with the reason -- the same shape as top's list, and put here for the same reason: the
// question gets asked, and a comment in the source is not where it gets asked.
var editorAbsent = []struct{ what, why string }{
	{"multiple buffers", "one file at a time; a second operand is refused rather than ignored"},
	{"replace", "search only. Replace needs a confirm-and-step loop this does not have"},
	{"mouse", "not wired up"},
	{"a configuration file", "the defaults are the whole of it"},
	{"soft wrap", "long lines scroll sideways. Highlighting needs one screen row to be one buffer line, because tview's line-start table for a wrapped row is unexported"},
	{"a second view", "no split; there is one buffer and one window"},
}
