package applets

import (
	"fmt"
	"os"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// Writing out and leaving, asked as nano and micro ask.
//
// ^X on a modified buffer said "Press the exit key again to discard them" and cleared the
// modified mark, so there was no way to save on the way out: a buffer was kept by ^O and then
// ^X, or lost by ^X twice. nano asks instead -- "Save modified buffer?", Y to write it, N to
// leave without, ^C to go back -- and Y offers the file's name to write to, which Enter
// accepts. ^O asks for the name the same way, so nano's ^O and Enter writes the file, where
// the Enter went into the buffer as a new line. micro asks its own question on ^Q and writes
// on ^S without one, as micro does.

// confirmCancelled is what a single-key prompt is answered with by Escape or ^C, which go back
// rather than answer. Not a key anyone can type as a letter, so `q` stays a letter.
const confirmCancelled rune = 0x1b

// writeOut is nano's ^O: the name to write to, the buffer's own offered, and then, written,
// whatever comes next -- leaving, for ^X.
func (v *editorView) writeOut(then func()) {
	if v.session.readOnly {
		v.setMessage("[red]%v[-]", errEditorReadOnly)
		return
	}
	v.askWith("File Name to Write: ", v.session.path, func(name string) {
		if name == "" {
			v.setMessage("Cancelled")
			return
		}
		v.writeNamed(name, then)
	})
}

// writeNamed writes to name, asking first when that is another file that exists already, as
// nano does: writing out under a new name is how a copy is made, and a copy made over a file
// that was there is a file lost.
func (v *editorView) writeNamed(name string, then func()) {
	if name != v.session.path && v.session.exists(name) {
		v.askKey(fmt.Sprintf("File %q exists; overwrite?  Y Yes  N No  ^C Cancel", name), func(answer rune) bool {
			switch answer {
			case 'y':
				v.writeTo(name, then)
			case 'n', confirmCancelled:
				v.setMessage("Cancelled")
			default:
				return false
			}
			return true
		})
		return
	}
	v.writeTo(name, then)
}

// writeTo writes the buffer to name and goes on to then. A write that fails stops there, with
// the reason, so a buffer that could not be written is never left behind by ^X.
func (v *editorView) writeTo(name string, then func()) {
	if err := v.session.saveAs(name, v.area.GetText()); err != nil {
		v.setMessage("[red]%v[-]", err)
		return
	}
	v.modified = false
	v.refreshTitle()
	v.setMessage("Wrote %s", v.session.path)
	if then != nil {
		then()
	}
}

// save is the write key: nano's asks for the name, micro's writes to the one it has.
func (v *editorView) save() {
	if v.keys.name == "nano" || v.session.path == "" {
		v.writeOut(nil)
		return
	}
	v.writeTo(v.session.path, nil)
}

// quit is the exit key. An unmodified buffer leaves at once; a modified one is asked about.
func (v *editorView) quit() {
	if !v.modified {
		v.application.Stop()
		return
	}
	question := "Save modified buffer?  Y Yes  N No  ^C Cancel"
	if v.keys.name == "micro" {
		question = fmt.Sprintf("Save changes to %s before closing? (y,n,esc)", v.displayName())
	}
	v.askKey(question, func(answer rune) bool {
		switch answer {
		case 'y':
			if v.keys.name == "micro" && v.session.path != "" {
				v.writeTo(v.session.path, v.application.Stop)
			} else {
				v.writeOut(v.application.Stop)
			}
		case 'n':
			v.application.Stop()
		case confirmCancelled:
			v.setMessage("Cancelled")
		default:
			return false
		}
		return true
	})
}

// askKey asks a single-key question, and asks it again for a key that answers nothing:
// guessing would be worse than waiting, and a prompt that takes every stray key for "no" is
// one people learn to distrust. answer reports whether the key answered.
func (v *editorView) askKey(question string, answer func(rune) bool) {
	var ask func()
	ask = func() {
		v.confirm = func(key rune) {
			if !answer(key) {
				ask()
			}
		}
		v.setMessage("%s", question)
	}
	ask()
}

// showLocation is nano's ^C at the buffer: where the cursor is. ^C never leaves.
func (v *editorView) showLocation() {
	lines, row := v.lines()
	_, column, _, _ := v.area.GetCursor()
	total := max(len(lines), 1)
	v.setMessage("line %d/%d (%d%%), col %d", row+1, total, (row+1)*100/total, column+1)
}

// displayName is the buffer's name as the title shows it.
func (v *editorView) displayName() string {
	if name := filepathBase(v.session.path); name != "" {
		return name
	}
	return "New Buffer"
}

// isControlKey reports whether event is Ctrl and letter, by either spelling: the Key constant
// a terminal sends, or the rune with ModCtrl the Windows console does (see editorChord).
func isControlKey(event *tcell.EventKey, letter rune, key tcell.Key) bool {
	if event.Key() == key {
		return true
	}
	return event.Key() == tcell.KeyRune && event.Modifiers()&^tcell.ModShift == tcell.ModCtrl &&
		unicode.ToLower(event.Rune()) == letter
}

// exists reports whether name is a file there already.
func (s *editorSession) exists(name string) bool {
	native, err := resolveHostPath(s.view, name)
	if err != nil {
		return false
	}
	_, err = os.Stat(native)
	return err == nil
}
