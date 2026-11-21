package applets

import "testing"

// `stty -g` prints the console's state as one word that `stty WORD` takes back, as GNU's prints
// its termios: here the input mode and the output mode, in hex, joined by a colon, the output's
// a dash when no output was a console. A script keeps it and gives it back -- atuin's bash init
// saves the terminal so at the start and puts it back around its search -- and never reads it.
func TestSttySaved_isAWordStttyTakesBack(t *testing.T) {
	for _, test := range []struct {
		in, out uint32
		hasOut  bool
		word    string
	}{
		{in: 0x1f7, out: 0x7, hasOut: true, word: "1f7:7"},
		{in: 0x87, word: "87:-"},
	} {
		word := sttySavedWord(test.in, test.out, test.hasOut)
		if word != test.word {
			t.Errorf("saved %#x, %#x as %q, want %q", test.in, test.out, word, test.word)
		}
		in, out, hasOut, ok := parseSttySaved(word)
		if !ok || in != test.in || hasOut != test.hasOut || (hasOut && out != test.out) {
			t.Errorf("%q read back as %#x, %#x, %v, %v", word, in, out, hasOut, ok)
		}
	}
	for _, setting := range []string{"echo", "-echo", "size", "1f7", "x:7", "1f7:7:3", ":7"} {
		if _, _, _, ok := parseSttySaved(setting); ok {
			t.Errorf("%q read as a saved state", setting)
		}
	}
}
