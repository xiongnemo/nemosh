package main

import "io"

// nextKey decodes one key, reading more bytes when the buffer holds only part
// of a sequence. A paste comes back whole, as one key; see readPaste.
func (e *lineEditor) nextKey() (key, error) {
	decoded, err := e.nextTypedKey()
	// Ctrl-H is a word or a character before any mode sees it; see lineedit_ctrlh.go.
	if decoded.kind == keyCtrlH {
		decoded.kind = e.ctrlHKind()
	}
	if err != nil || decoded.kind != keyPasteStart {
		return decoded, err
	}
	return e.readPaste()
}

// nextTypedKey is one key as the terminal sent it.
func (e *lineEditor) nextTypedKey() (key, error) {
	for {
		// The line feed of a CRLF that was split across two reads. decodeKey joins the
		// pair when it has both, which is the usual case, but a read boundary can fall
		// between them -- and a bare line feed has to stay an Enter, so the two cannot be
		// told apart without remembering that the last key was a carriage return.
		if e.afterCarriageReturn && len(e.pending) > 0 {
			if e.pending[0] == '\n' {
				e.pending = e.pending[1:]
			}
			e.afterCarriageReturn = false
		}
		decode := decodeKey
		if e.vi.on {
			decode = decodeViKey
		}
		// What bind bound is read before the editor's own keys; see lineedit_bind.go.
		bound, done, wait := e.boundKey()
		if done {
			return bound, nil
		}
		// An Escape nothing follows soon is the key itself, in vi mode.
		if !wait && e.vi.on && len(e.pending) == 1 && e.pending[0] == 0x1b && (e.inputWaiting == nil || !e.inputWaiting(escapeWait)) {
			e.pending = e.pending[:0]
			return key{kind: keyEscape}, nil
		}
		if decoded, consumed := decode(e.pending); !wait && decoded.kind != keyIncomplete {
			e.afterCarriageReturn = decoded.kind == keyEnter && consumed == 1 && e.pending[0] == '\r'
			e.enterFromPair = decoded.kind == keyEnter && consumed == 2
			e.pending = e.pending[consumed:]
			return decoded, nil
		}
		chunk := make([]byte, 64)
		count, err := e.input.Read(chunk)
		if count > 0 {
			e.pending = append(e.pending, chunk[:count]...)
			continue
		}
		if err == nil {
			err = io.EOF
		}
		return key{}, err
	}
}

// readPaste gathers what a terminal in bracketed-paste mode sends between its marks into one
// key, so that it goes into the line as it is: a pasted newline is a newline in the line
// rather than Enter, and a pasted Tab a tab rather than a completion. Nothing runs until Enter
// is pressed after it, which is what bash and zsh do with a paste.
//
// A paste is text, so a key in it that is no character -- an arrow, a control key -- is left
// out. Input that ends inside a paste still delivers what came.
func (e *lineEditor) readPaste() (key, error) {
	var text []rune
	for {
		decoded, err := e.nextTypedKey()
		if err != nil || decoded.kind == keyPasteEnd {
			return key{kind: keyPaste, text: string(text)}, nil
		}
		switch decoded.kind {
		case keyRune:
			text = append(text, decoded.value)
		case keyEnter:
			text = append(text, '\n')
		case keyTab:
			text = append(text, '\t')
		}
	}
}

// insertText inserts each character of text at the cursor. A newline and a tab go in as they
// are, which insert refuses from the keyboard: a pasted script needs both, and a tab can be
// syntax -- `<<-` strips leading tabs -- so it cannot become spaces. The drawing allows for
// both; see layoutPosition.
func (b *lineBuffer) insertText(text string) {
	for _, r := range text {
		if r != '\n' && r != '\t' {
			b.insert(r)
			continue
		}
		b.runes = append(b.runes, 0)
		copy(b.runes[b.cursor+1:], b.runes[b.cursor:])
		b.runes[b.cursor] = r
		b.cursor++
	}
}
