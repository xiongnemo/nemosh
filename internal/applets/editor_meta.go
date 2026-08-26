package applets

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// The Escape prefix, which is what `M-G` is on a terminal.
//
// nano documents go-to-line as `^_` or `M-G`. Neither reaches this editor on Windows as
// written: `^_` is mangled into Ctrl+Backspace by the console path (see editor_keys.go),
// and Alt with a letter is reported by the Windows console with no character at all, so
// tcell drops the event before anyone sees it (console_win.go:741-744). Escape then the
// letter is the third spelling, it is what `M-` has always meant on a terminal, and both
// halves are ordinary keys that cannot fail to arrive.

// handleMetaKey deals with Escape and the key after it, and answers what handleKey
// should return -- or nil to say it is not a meta press and the ordinary path applies.
//
// A pointer to the answer rather than a (value, bool) pair because the answer is itself
// a pointer that is meaningfully nil: swallowing a key is `return nil`.
func (v *editorView) handleMetaKey(event *tcell.EventKey) **tcell.EventKey {
	swallow := (*tcell.EventKey)(nil)
	if v.meta {
		v.meta = false
		// Escape twice is a way to type a literal Escape, and is also the way out of a
		// prefix pressed by accident.
		if event.Key() == tcell.KeyEscape {
			v.setMessage("")
			return &swallow
		}
		if action := v.keys.lookup(metaChordOf(event)); action != editorNothing {
			v.runAction(action)
			return &swallow
		}
		v.setMessage("[yellow]M-%s is not bound[-]", strings.ToUpper(string(event.Rune())))
		return &swallow
	}
	if event.Key() == tcell.KeyEscape {
		v.meta = true
		// Shown, because a prefix that waits silently is indistinguishable from a key
		// that did nothing.
		v.setMessage("M-")
		return &swallow
	}
	return nil
}

// metaChordOf rewrites Escape-then-key as the Alt chord it means, so that one binding
// table answers both spellings.
func metaChordOf(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyRune {
		return event
	}
	return tcell.NewEventKey(tcell.KeyRune, event.Rune(), event.Modifiers()|tcell.ModAlt)
}
