package main

import goruntime "runtime"

// Ctrl-H, 0x08.
//
// Windows Terminal sends it for Ctrl+Backspace, which deletes the word before the cursor in
// PowerShell and every Windows edit box, and a Unix terminal set to erase with ^H sends it for
// Backspace; the two cannot be told apart, so the platform decides: a word on Windows, a
// character elsewhere. NEMOSH_CTRL_H=backspace or word says which anywhere, asked at each press
// as NEMOSH_CTRL_Z is. busybox-w32's editor deletes a character on Ctrl+Backspace; Alt+Backspace
// and Ctrl-W delete a word in both.

// ctrlHKind is the key a Ctrl-H is: keyDeleteWord, Ctrl-W's, so that what it takes goes to the
// kill ring, or keyBackspace.
func (e *lineEditor) ctrlHKind() keyKind {
	setting := ""
	if e.setting != nil {
		setting, _ = e.setting("NEMOSH_CTRL_H")
	}
	switch {
	case setting == "word":
		return keyDeleteWord
	case setting == "backspace":
		return keyBackspace
	case goruntime.GOOS == "windows":
		return keyDeleteWord
	}
	return keyBackspace
}
