package applets

import "github.com/gdamore/tcell/v2"

// The keys vi reads, as busybox's read_key gives them: a character as itself, a control
// character as itself -- ^R is 18 -- and a key that is neither as a negative number, so a
// command, and what `.` replays of one, is a []rune.
const (
	viEscape    rune = 0x1b
	viEnter     rune = '\r'
	viBackspace rune = 0x7f
)

const (
	viKeyUp rune = -(iota + 1)
	viKeyDown
	viKeyLeft
	viKeyRight
	viKeyHome
	viKeyEnd
	viKeyPageUp
	viKeyPageDown
	viKeyDelete
	viKeyInsert
)

// isViBackspace is busybox's isbackspace: ^H and DEL, both, whichever the terminal sends.
func isViBackspace(key rune) bool { return key == 8 || key == viBackspace }

// viKeysOf is what a key event means to vi: none, one key, or two.
//
// tcell 2.13 reads a Windows console as a terminal (tscreen_win.go), measured with a console
// robot: an Escape and the key typed within 50 ms of it arrive as that key with Alt, as tcell's
// input.go parses a terminal's, so a quick `<Esc>:wq` is Alt+: -- two keys here, Escape and
// the colon. Ctrl with a letter is KeyCtrlA and on, and a control character typed some other
// way is its own Key below 32; both are the control character.
func viKeysOf(event *tcell.EventKey) []rune {
	key, character, modifiers := event.Key(), event.Rune(), event.Modifiers()
	switch {
	case key == tcell.KeyRune && modifiers&tcell.ModAlt != 0 && modifiers&tcell.ModCtrl == 0:
		return []rune{viEscape, character}
	case key == tcell.KeyRune:
		return []rune{character}
	case key >= tcell.KeyCtrlSpace && key <= tcell.KeyCtrlUnderscore:
		return []rune{rune(key - tcell.KeyCtrlSpace)}
	case key < 32 || key == tcell.KeyDEL:
		return []rune{rune(key)}
	}
	if special, ok := viSpecialKey(key); ok {
		return []rune{special}
	}
	return nil
}

func viSpecialKey(key tcell.Key) (rune, bool) {
	switch key {
	case tcell.KeyUp:
		return viKeyUp, true
	case tcell.KeyDown:
		return viKeyDown, true
	case tcell.KeyLeft:
		return viKeyLeft, true
	case tcell.KeyRight:
		return viKeyRight, true
	case tcell.KeyHome:
		return viKeyHome, true
	case tcell.KeyEnd:
		return viKeyEnd, true
	case tcell.KeyPgUp:
		return viKeyPageUp, true
	case tcell.KeyPgDn:
		return viKeyPageDown, true
	case tcell.KeyDelete:
		return viKeyDelete, true
	case tcell.KeyInsert:
		return viKeyInsert, true
	}
	return 0, false
}

// viKeyName is a key as a message names it, busybox's print_literal: a control character as
// ^X, DEL as ^?, and a key with no character by its name.
func viKeyName(key rune) string {
	switch {
	case key < 0:
		return [...]string{"<Up>", "<Down>", "<Left>", "<Right>", "<Home>", "<End>", "<PageUp>",
			"<PageDown>", "<Del>", "<Insert>"}[-key-1]
	case key < ' ':
		return "^" + string(key|'@')
	case key == viBackspace:
		return "^?"
	}
	return string(key)
}
