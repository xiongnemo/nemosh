package main

// Ctrl-H, 0x08.
//
// Windows Terminal sends it for Ctrl+Backspace, which deletes the word before the cursor in
// PowerShell and every Windows edit box, so it deletes a word here. A terminal set to erase with
// ^H sends it for Backspace, and the two cannot be told apart, so NEMOSH_CTRL_H=backspace makes it
// one, asked at each press as NEMOSH_CTRL_Z is. busybox-w32's editor deletes a character on
// Ctrl+Backspace; Alt+Backspace and Ctrl-W delete a word in both.

// ctrlHKind is the key a Ctrl-H is: keyDeleteWord, Ctrl-W's, so that what it takes goes to the
// kill ring, or keyBackspace.
func (e *lineEditor) ctrlHKind() keyKind {
	setting := ""
	if e.setting != nil {
		setting, _ = e.setting("NEMOSH_CTRL_H")
	}
	if setting == "backspace" {
		return keyBackspace
	}
	return keyDeleteWord
}
