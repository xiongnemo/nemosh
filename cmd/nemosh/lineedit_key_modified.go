package main

import "strconv"

// decodeModifiedKey reads a key a terminal sends with its modifiers, CSI number;modifier final,
// from the `;` at semicolon. The modifier is one more than a mask of Shift 1, Alt 2, Ctrl 4 and
// Meta 8: CSI 1;5D is Ctrl-Left, 1;2D Shift-Left, 3;5~ Ctrl-Delete. Every modified arrow moved by a
// word, Shift-Left too, and a modified Delete did nothing. Now an arrow moves by a word with
// Ctrl, Alt or Meta and by a character without, and Delete deletes the word after the cursor
// with Ctrl or Alt, as Ctrl+Delete does in PowerShell and a Windows edit box, and a character
// otherwise. busybox-w32's editor takes Ctrl and Alt arrows by word and ignores the others.
func decodeModifiedKey(buffer []byte, semicolon int) (key, int) {
	end := semicolon + 1
	for end < len(buffer) && buffer[end] >= '0' && buffer[end] <= '9' {
		end++
	}
	if end >= len(buffer) {
		return key{kind: keyIncomplete}, 0
	}
	modifier, _ := strconv.Atoi(string(buffer[semicolon+1 : end]))
	byWord := (modifier-1)&(2|4|8) != 0
	pick := func(word, character keyKind) key {
		if byWord {
			return key{kind: word}
		}
		return key{kind: character}
	}
	switch {
	case buffer[end] == 'D':
		return pick(keyWordLeft, keyLeft), end + 1
	case buffer[end] == 'C':
		return pick(keyWordRight, keyRight), end + 1
	case buffer[end] == '~' && string(buffer[2:semicolon]) == "3":
		return pick(keyDeleteWordForward, keyDelete), end + 1
	}
	return key{kind: keyUnknown}, end + 1
}
