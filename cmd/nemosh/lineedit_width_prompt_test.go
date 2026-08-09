package main

import "testing"

// A prompt's width is what it draws, not how many bytes it takes. Colour is
// escape sequences, and every byte of `[1;31m` is printable on its own -- only
// the ESC that introduces them is a control character.
//
// Getting this wrong is not subtle: the editor places the cursor by moving it
// promptColumns to the right of the line start, so an over-count pushes every
// keystroke that far past the prompt. A coloured `$ ` measured 11 instead of 2
// left nine blank cells before the first character typed.
func TestPromptColumns_ignoresEscapeSequences(t *testing.T) {
	for _, test := range []struct {
		name   string
		prompt string
		want   int
	}{
		{name: "plain", prompt: "$ ", want: 2},
		{name: "the default coloured symbol", prompt: "\033[1;31m$\033[0m ", want: 2},
		{name: "several sequences", prompt: "\033[1;34ma\033[0m\033[2mb\033[0m", want: 2},
		{name: "colour around a wide character", prompt: "\033[1;32m你\033[0m ", want: 3},
		{name: "no colour at all", prompt: "nemo@host ", want: 10},
		{name: "an escape with no final byte is not counted", prompt: "\033[1;31", want: 0},
		{name: "empty", prompt: "", want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := promptColumns(test.prompt); got != test.want {
				t.Fatalf("promptColumns(%q) = %d, want %d", test.prompt, got, test.want)
			}
		})
	}
}

// The whole prompt is measured by its last row, and that row may carry colour
// too -- the default prompt ends with a newline and then a coloured symbol.
func TestPromptColumns_measuresOnlyTheLastRow(t *testing.T) {
	prompt := "\033[1;34m# nemo\033[0m in \033[0;33m/tmp\033[0m\n\033[1;31m$\033[0m "
	if got := promptColumns(lastPromptLine(prompt)); got != 2 {
		t.Fatalf("promptColumns of the last row = %d, want 2", got)
	}
}
