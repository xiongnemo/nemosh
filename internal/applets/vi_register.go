package applets

import "strings"

// Registers and marks, busybox's FEATURE_VI_YANKMARK: "a to "z name a register for the next
// command, which otherwise uses D, busybox's default; m{a-z} marks a place, '{a-z} goes to its
// line and `{a-z} to the place itself, vim's; '' and `` go back to where the last jump left.
// "A to "Z append to their register, vim's; busybox's took them as "a to "z.

// viRegister is what a register holds: text, a newline ending each line of a linewise one.
type viRegister struct {
	text     []rune
	linewise bool
}

// viMark is a place a mark or the last jump keeps.
type viMark struct {
	pos viPos
	set bool
}

// viDefaultRegister is busybox's D, the one a command uses when none is named.
const viDefaultRegister = 26

// registerName is busybox's what_reg.
func registerName(index int) rune {
	if index < 26 {
		return rune('a' + index)
	}
	return 'D'
}

// chooseRegister is the key after ", which names the register the next command uses.
func (e *viEditor) chooseRegister(key rune) {
	switch {
	case key >= 'a' && key <= 'z':
		e.register, e.appending = int(key-'a'), false
	case key >= 'A' && key <= 'Z':
		e.register, e.appending = int(key-'A'), true
	default:
		e.beep()
	}
}

// store puts text in the register the command named, or in D, and in D as well, and says so
// as busybox's yank_status does when operation is not "".
func (e *viEditor) store(operation string, text []rune, linewise bool) {
	index := e.register
	register := viRegister{text: text, linewise: linewise}
	if e.appending {
		held := e.registers[index]
		register = viRegister{text: append(append([]rune(nil), held.text...), text...), linewise: held.linewise || linewise}
	}
	e.registers[index] = register
	if index != viDefaultRegister {
		e.registers[viDefaultRegister] = register
	}
	if operation != "" {
		e.sayTaken(operation, register.text, 1, index)
	}
}

// sayTaken is busybox's yank_status: `Yank 1 lines (2 chars) from [D]`.
func (e *viEditor) sayTaken(operation string, text []rune, times, index int) {
	lines := strings.Count(string(text), "\n")
	e.say(false, "%s %d lines (%d chars) from [%c]", operation, lines*times, len(text)*times, registerName(index))
}

// mark is the key after m.
func (e *viEditor) mark(key rune) {
	key |= 0x20
	if key < 'a' || key > 'z' {
		e.beep()
		return
	}
	e.marks[key-'a'] = viMark{pos: e.cursor(), set: true}
}

// markPlace is where ' or ` with key goes, and whether there is anywhere.
func (e *viEditor) markPlace(key rune) (viPos, bool) {
	var mark viMark
	switch {
	case key == '\'' || key == '`':
		mark = e.context
	case key|0x20 >= 'a' && key|0x20 <= 'z':
		mark = e.marks[key|0x20-'a']
	}
	if !mark.set || mark.pos.row >= len(e.lines) {
		return viPos{}, false
	}
	mark.pos.col = min(mark.pos.col, max(0, len(e.lines[mark.pos.row])-1))
	return mark.pos, true
}

// goToMark is ' and `: to the mark's line, at its first character that is not a blank, or to
// the place itself.
func (e *viEditor) goToMark(command, key rune) {
	place, ok := e.markPlace(key)
	if !ok {
		e.beep()
		e.failed = true
		return
	}
	from := e.cursor()
	e.setCursor(place)
	if command == '\'' {
		e.col = e.firstNonBlank(e.row)
	}
	e.context = viMark{pos: from, set: true}
}

// isJump is busybox's check_context list, vim's jumps: the motions after which a quote typed
// twice goes back.
func isJump(key rune) bool { return strings.ContainsRune("%{}'`GHLMnN/?", key) }
