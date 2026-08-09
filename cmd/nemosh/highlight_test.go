package main

import (
	"strings"
	"testing"
)

// neverKnows is an oracle that recognises nothing, for tests about the theme
// rather than about what exists.
func neverKnows(string) commandStanding { return standingUnknown }

// hasParameter reports whether an SGR parameter is present as itself.
// Substring matching would call `34` an underline because it contains `4`.
func hasParameter(style, want string) bool {
	for _, parameter := range strings.Split(style, ";") {
		if parameter == want {
			return true
		}
	}
	return false
}

// The word being edited is underlined until a blank ends it, which is the
// visible answer to "what will Tab act on".
func TestHighlight_underlinesTheWordBeingEdited(t *testing.T) {
	for _, test := range []struct {
		name   string
		typed  string
		column int
		want   string
	}{
		{name: "the command being typed", typed: "ech", column: 2, want: "ech"},
		{name: "an operand being typed", typed: "echo hell", column: 7, want: "hell"},
		{name: "nothing once a blank ends it", typed: "echo hello ", column: 2, want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given / When
			screen, _ := typedScreen(t, 60, test.typed)

			// Then: the underline attribute is present exactly under that word.
			var underlined []string
			for column := 0; column < 60; column++ {
				if hasParameter(screen.styleAt(0, column), "4") {
					underlined = append(underlined, string(screen.rows[0][column]))
				}
			}
			if got := strings.Join(underlined, ""); got != test.want {
				t.Fatalf("underlined %q, want %q", got, test.want)
			}
		})
	}
}

// A command this shell carries is drawn in one colour and a name it does not in
// another. The distinction is what the capability table answers, so completion
// and this can never disagree about what exists.
func TestHighlight_coloursACommandByWhetherItExists(t *testing.T) {
	for _, test := range []struct {
		name  string
		typed string
		want  string
	}{
		{name: "an applet", typed: "echo x", want: "32"},
		{name: "a builtin", typed: "export x", want: "32"},
		{name: "no such command", typed: "nosuchcmd x", want: "31"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given / When
			screen, _ := typedScreen(t, 60, test.typed)

			// Then: the first character of the command word carries the colour.
			// The word being edited is the operand here, so the command word is
			// coloured and not underlined.
			if got := screen.styleAt(0, 2); got != test.want {
				t.Fatalf("command style = %q, want %q", got, test.want)
			}
		})
	}
}

// An option is drawn by whether the command accepts it, from the same table Tab
// offers from.
func TestHighlight_coloursAnOptionByWhetherItIsAccepted(t *testing.T) {
	for _, test := range []struct {
		name  string
		typed string
		want  string
	}{
		{name: "accepted", typed: "ls -a x", want: "36"},
		{name: "accepted cluster", typed: "ls -al x", want: "36"},
		{name: "one bad letter spoils the cluster", typed: "ls -aZ x", want: "33"},
		{name: "unknown short", typed: "ls -Z x", want: "33"},
		{name: "accepted long", typed: "ls --color x", want: "36"},
		{name: "accepted long with a value", typed: "ls --color=auto x", want: "36"},
		{name: "unknown long", typed: "ls --colour x", want: "33"},
		// An external program's options are not in the table, so no verdict is
		// invented for them.
		{name: "unknown command means no verdict", typed: "nosuchcmd -Z x", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given / When
			screen, _ := typedScreen(t, 60, test.typed)

			// Then: the option word begins after the command and one blank.
			column := 2 + strings.Index(test.typed, " -") + 1
			if got := screen.styleAt(0, column); got != test.want {
				t.Fatalf("option style at column %d = %q, want %q", column, got, test.want)
			}
		})
	}
}

// Styling is decoration and must never reach the buffer, or the line submitted
// would carry escapes and the column arithmetic would count them as text.
func TestHighlight_neverEntersTheBuffer(t *testing.T) {
	// Given / When
	_, editor := typedScreen(t, 60, "ls -a")

	// Then
	if got := editor.buffer.String(); got != "ls -a" {
		t.Fatalf("buffer = %q, want exactly what was typed", got)
	}
	if strings.ContainsRune(editor.buffer.String(), 0x1b) {
		t.Fatal("an escape reached the buffer")
	}
}

// With colour unavailable the whole thing turns itself off rather than
// degrading: a grey suggestion rendered as ordinary text would put characters on
// screen that are not in the line, and Enter would run something shorter than
// what is visible.
func TestTheme_disablesItselfWhenColourIsUnavailable(t *testing.T) {
	for _, test := range []struct {
		name        string
		environment map[string]string
		want        bool
	}{
		{name: "an ordinary terminal", environment: map[string]string{"TERM": "xterm-256color"}, want: true},
		{name: "NO_COLOR at any value", environment: map[string]string{"NO_COLOR": ""}, want: false},
		{name: "a dumb terminal", environment: map[string]string{"TERM": "dumb"}, want: false},
		{name: "asked for never", environment: map[string]string{"NEMOSH_COLOR": "never"}, want: false},
		{name: "asked for always despite NO_COLOR", environment: map[string]string{"NEMOSH_COLOR": "always", "NO_COLOR": "1"}, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			lookup := func(name string) (string, bool) {
				value, ok := test.environment[name]
				return value, ok
			}

			// When
			styled := newTheme(lookup)

			// Then
			if styled.enabled != test.want {
				t.Fatalf("enabled = %v, want %v", styled.enabled, test.want)
			}
			if !test.want && styled.paint("ls -a", 5, "bcd", neverKnows) != "ls -a" {
				t.Fatal("a disabled theme still painted something")
			}
			if !test.want && styled.suggests() {
				t.Fatal("a disabled theme still offers suggestions")
			}
		})
	}
}
