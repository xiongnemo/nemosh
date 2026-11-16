package main

import "testing"

// Ctrl+Backspace deletes the word before the cursor, as it does in PowerShell and every Windows
// edit box, and Ctrl+Delete the one after it. Windows Terminal sends Ctrl+Backspace as 0x08,
// which is also Ctrl-H, and a terminal set to erase with ^H sends it for Backspace, so
// NEMOSH_CTRL_H=backspace makes it one. busybox-w32's editor deletes a character on
// Ctrl+Backspace and nothing on Ctrl+Delete.
func TestLineEditor_ctrlBackspaceDeletesAWordOnWindows(t *testing.T) {
	for _, test := range []struct {
		name     string
		keys     string
		settings map[string]string
		want     string
	}{
		{name: "by default", keys: "echo hello world\x08\r", want: "echo hello "},
		{name: "NEMOSH_CTRL_H=backspace", keys: "echo hello world\x08\r", settings: map[string]string{"NEMOSH_CTRL_H": "backspace"}, want: "echo hello worl"},
		{name: "NEMOSH_CTRL_H=word", keys: "echo hello world\x08\r", settings: map[string]string{"NEMOSH_CTRL_H": "word"}, want: "echo hello "},
		{name: "a word deleted is yanked back", keys: "echo hello\x08X\x19\r", settings: map[string]string{"NEMOSH_CTRL_H": "word"}, want: "echo Xhello"},
		{name: "ctrl-delete deletes the word after", keys: "echo hello world\x01\x1b[3;5~\r", want: " hello world"},
		{name: "shift-left moves one character", keys: "abc\x1b[1;2DX\r", want: "abXc"},
		{name: "ctrl-left still moves a word", keys: "echo abc\x1b[1;5DX\r", want: "echo Xabc"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			line, err := editLineWith(t, test.keys, test.settings)

			// Then
			if err != nil || line != test.want {
				t.Fatalf("keys %q gave %q, %v; want %q", test.keys, line, err, test.want)
			}
		})
	}
}
