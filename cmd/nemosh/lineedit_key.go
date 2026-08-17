package main

import "unicode/utf8"

type keyKind int

const (
	// keyIncomplete means the bytes so far are a prefix of something longer.
	// A terminal is free to deliver an escape sequence one byte at a time, so
	// guessing here would turn a slow arrow key into a stray Escape.
	keyIncomplete keyKind = iota
	keyUnknown
	keyRune
	keyEnter
	keyBackspace
	keyDelete
	keyTab
	keyInterrupt
	keyEndOfInput
	keyUp
	keyDown
	keyLeft
	keyRight
	keyHome
	keyEnd
	keyClearLine
	keyDeleteWord
	keyDeleteWordForward
	keyWordLeft
	keyWordRight
	keyClearScreen
	keyReverseSearch
	// keyAbort is Ctrl-G, which readline binds to abort. It only means anything
	// during an incremental search, where it is the escape hatch that puts the
	// line back.
	keyAbort
)

type key struct {
	kind  keyKind
	value rune
}

// decodeKey reads one key from the front of buffer, returning how many bytes it
// consumed. A zero count with keyIncomplete means "read more first".
//
// Ctrl-D and Ctrl-Z both report end of input. Ctrl-D is the Unix spelling and
// Ctrl-Z the Windows one; a shell that runs on Windows but is used by people
// with Unix habits should answer to either, and neither reaches a cooked-mode
// reader as anything but a stray byte to run as a command.
func decodeKey(buffer []byte) (key, int) {
	if len(buffer) == 0 {
		return key{kind: keyIncomplete}, 0
	}
	switch buffer[0] {
	case '\r', '\n':
		return key{kind: keyEnter}, 1
	case 0x7f, 0x08:
		return key{kind: keyBackspace}, 1
	case '\t':
		return key{kind: keyTab}, 1
	case 0x03:
		return key{kind: keyInterrupt}, 1
	case 0x04, 0x1a:
		return key{kind: keyEndOfInput}, 1
	case 0x01:
		return key{kind: keyHome}, 1
	case 0x05:
		return key{kind: keyEnd}, 1
	case 0x07:
		return key{kind: keyAbort}, 1
	case 0x0c:
		return key{kind: keyClearScreen}, 1
	case 0x12:
		return key{kind: keyReverseSearch}, 1
	case 0x15:
		return key{kind: keyClearLine}, 1
	case 0x17:
		return key{kind: keyDeleteWord}, 1
	case 0x1b:
		return decodeEscapeSequence(buffer)
	}
	if buffer[0] < 0x20 {
		return key{kind: keyUnknown}, 1
	}
	r, size := utf8.DecodeRune(buffer)
	if r == utf8.RuneError && size <= 1 {
		// Either a real replacement character split across reads, or invalid
		// input. Wait for more before deciding.
		if !utf8.FullRune(buffer) {
			return key{kind: keyIncomplete}, 0
		}
		return key{kind: keyUnknown}, 1
	}
	return key{kind: keyRune, value: r}, size
}

// decodeEscapeSequence handles the CSI forms a terminal sends for the arrows
// and the navigation block.
func decodeEscapeSequence(buffer []byte) (key, int) {
	if len(buffer) < 2 {
		return key{kind: keyIncomplete}, 0
	}
	if buffer[1] != '[' && buffer[1] != 'O' {
		// A Meta key. readline's word bindings live here; bash reports them as
		// kill-word "\ed", backward-kill-word "\e\C-h" and "\e\C-?",
		// backward-word "\eb", forward-word "\ef".
		switch buffer[1] {
		case 'd':
			return key{kind: keyDeleteWordForward}, 2
		case 0x7f, 0x08:
			return key{kind: keyDeleteWord}, 2
		case 'b':
			return key{kind: keyWordLeft}, 2
		case 'f':
			return key{kind: keyWordRight}, 2
		}
		// Anything else Meta is skipped whole rather than inserted, so an
		// unbound Alt key cannot leave its letter in the line.
		return key{kind: keyUnknown}, 2
	}
	if len(buffer) < 3 {
		return key{kind: keyIncomplete}, 0
	}
	switch buffer[2] {
	case 'A':
		return key{kind: keyUp}, 3
	case 'B':
		return key{kind: keyDown}, 3
	case 'C':
		return key{kind: keyRight}, 3
	case 'D':
		return key{kind: keyLeft}, 3
	case 'H':
		return key{kind: keyHome}, 3
	case 'F':
		return key{kind: keyEnd}, 3
	}
	// A parameterised form: digits and then a final byte.
	end := 2
	for end < len(buffer) && buffer[end] >= '0' && buffer[end] <= '9' {
		end++
	}
	if end >= len(buffer) {
		return key{kind: keyIncomplete}, 0
	}
	if buffer[end] == ';' {
		// A modified arrow: CSI 1;5D is Ctrl-Left, CSI 1;5C is Ctrl-Right.
		modifier := end + 1
		for modifier < len(buffer) && buffer[modifier] >= '0' && buffer[modifier] <= '9' {
			modifier++
		}
		if modifier >= len(buffer) {
			return key{kind: keyIncomplete}, 0
		}
		switch buffer[modifier] {
		case 'D':
			return key{kind: keyWordLeft}, modifier + 1
		case 'C':
			return key{kind: keyWordRight}, modifier + 1
		}
		return key{kind: keyUnknown}, modifier + 1
	}
	if buffer[end] != '~' {
		return key{kind: keyUnknown}, end + 1
	}
	switch string(buffer[2:end]) {
	case "1", "7":
		return key{kind: keyHome}, end + 1
	case "3":
		return key{kind: keyDelete}, end + 1
	case "4", "8":
		return key{kind: keyEnd}, end + 1
	}
	return key{kind: keyUnknown}, end + 1
}
