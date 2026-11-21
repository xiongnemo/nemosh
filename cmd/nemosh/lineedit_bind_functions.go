package main

import "unicode/utf8"

// readline's functions the editor had no key for until bind and macros reached them -- fzf's
// Alt-C types `\e\C-e` to expand a command substitution it put in the line, and atuin's keys
// end in accept-line after vi-movement-mode -- with the keys bash has them on.
const (
	keyRedrawLine keyKind = iota + 1000
	keySetMark
	keyExchangeMark
	keyShellExpandLine
	keyViEditingMode
	keyEmacsEditingMode
	keyViMovementMode
	keyViInsertionMode
)

// readlineOnlyKey does what one of those keys asks, and reports whether it was one.
func (e *lineEditor) readlineOnlyKey(prompt string, k key) bool {
	switch k.kind {
	case keyRedrawLine:
		e.restorePrompt(prompt)
	case keySetMark:
		e.mark = e.buffer.cursor
	case keyExchangeMark:
		e.mark, e.buffer.cursor = e.buffer.cursor, min(e.mark, len(e.buffer.runes))
	case keyShellExpandLine:
		if e.expandLine != nil {
			if line, ok := e.expandLine(e.buffer.String()); ok {
				e.buffer.runes = []rune(line)
				e.buffer.cursor = len(e.buffer.runes)
			}
		}
	case keyViEditingMode, keyEmacsEditingMode:
		e.vi = viState{on: k.kind == keyViEditingMode}
		if e.editingMode != nil {
			e.editingMode(e.vi.on)
		}
	case keyViMovementMode:
		if e.vi.on && !e.vi.command {
			e.vi.command = true
			e.buffer.moveLeft()
		}
	case keyViInsertionMode:
		e.vi.command = false
	default:
		return false
	}
	return true
}

// functionKey is the key a readline function is in this editor, for a sequence bind bound to it;
// pressed is the sequence, which self-insert puts in. Every function runtime's readlineActions
// lists is here, and a test holds the two together and decodes each default key to its function.
func functionKey(name, pressed string) (key, bool) {
	switch name {
	case "abort":
		return key{kind: keyAbort}, true
	case "accept-line":
		return key{kind: keyEnter}, true
	case "backward-char":
		return key{kind: keyLeft}, true
	case "backward-delete-char":
		return key{kind: keyBackspace}, true
	case "backward-kill-word", "unix-word-rubout":
		return key{kind: keyDeleteWord}, true
	case "backward-word":
		return key{kind: keyWordLeft}, true
	case "beginning-of-line":
		return key{kind: keyHome}, true
	case "capitalize-word":
		return key{kind: keyWordCase, value: 'c'}, true
	case "clear-screen":
		return key{kind: keyClearScreen}, true
	case "complete":
		return key{kind: keyTab}, true
	case "delete-char":
		return key{kind: keyDelete}, true
	case "downcase-word":
		return key{kind: keyWordCase, value: 'l'}, true
	case "edit-and-execute-command":
		return key{kind: keyEditAndExecute}, true
	case "end-of-line":
		return key{kind: keyEnd}, true
	case "forward-char":
		return key{kind: keyRight}, true
	case "forward-word":
		return key{kind: keyWordRight}, true
	case "history-search-backward":
		return key{kind: keyHistoryPrefixBackward}, true
	case "history-search-forward":
		return key{kind: keyHistoryPrefixForward}, true
	case "kill-line":
		return key{kind: keyKillToEnd}, true
	case "kill-word":
		return key{kind: keyDeleteWordForward}, true
	case "next-history":
		return key{kind: keyDown}, true
	case "previous-history":
		return key{kind: keyUp}, true
	case "reverse-search-history":
		return key{kind: keyReverseSearch}, true
	case "revert-line":
		return key{kind: keyRevertLine}, true
	case "self-insert":
		r, _ := utf8.DecodeLastRuneInString(pressed)
		return key{kind: keyRune, value: r}, r != utf8.RuneError
	case "transpose-chars":
		return key{kind: keyTranspose}, true
	case "undo":
		return key{kind: keyUndo}, true
	case "unix-line-discard":
		return key{kind: keyClearLine}, true
	case "upcase-word":
		return key{kind: keyWordCase, value: 'u'}, true
	case "yank":
		return key{kind: keyYank}, true
	case "yank-last-arg":
		return key{kind: keyYankLastArg}, true
	case "yank-pop":
		return key{kind: keyYankPop}, true
	case "redraw-current-line":
		return key{kind: keyRedrawLine}, true
	case "set-mark":
		return key{kind: keySetMark}, true
	case "exchange-point-and-mark":
		return key{kind: keyExchangeMark}, true
	case "shell-expand-line":
		return key{kind: keyShellExpandLine}, true
	case "vi-editing-mode":
		return key{kind: keyViEditingMode}, true
	case "emacs-editing-mode":
		return key{kind: keyEmacsEditingMode}, true
	case "vi-movement-mode":
		return key{kind: keyViMovementMode}, true
	case "vi-insertion-mode":
		return key{kind: keyViInsertionMode}, true
	}
	return key{}, false
}
