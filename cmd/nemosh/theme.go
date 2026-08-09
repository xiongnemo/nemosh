package main

import "strings"

// theme decides whether the line is drawn with any styling at all, and in what.
//
// The gate is not a nicety. A suggestion is drawn in grey *after* the cursor,
// and if grey does not render it appears as ordinary text -- the screen then
// shows characters that are not in the line, and Enter runs something shorter
// than what is visible. That is worse than having no suggestion, so an absent
// capability turns the feature off rather than degrading it.
//
// The same switch covers highlighting, because the two share one question: can
// this terminal be trusted with an SGR escape.
type theme struct {
	enabled bool
	colours palette
}

// newTheme reads the environment the way the ecosystem has settled on.
//
//   - NO_COLOR set to anything at all disables colour. That is the whole of the
//     no-color.org convention: presence, not value.
//   - TERM=dumb means a terminal that cannot be relied on for escapes.
//   - NEMOSH_COLOR=never or =always is this shell's own override, spelled with
//     the words `ls --color` already accepts, so there is one vocabulary.
func newTheme(lookupEnv func(string) (string, bool)) theme {
	styled := theme{enabled: true, colours: defaultPalette()}
	if value, ok := lookupEnv("NEMOSH_COLOR"); ok {
		switch strings.ToLower(value) {
		case "always", "force", "yes":
			return styled
		case "never", "none", "no":
			return theme{}
		}
	}
	if _, ok := lookupEnv("NO_COLOR"); ok {
		return theme{}
	}
	if value, _ := lookupEnv("TERM"); value == "dumb" {
		return theme{}
	}
	return styled
}

// paint renders the line, and the suggestion after it, as the terminal should
// receive them.
//
// Returns the drawable text only. What it deliberately does not return is any
// measurement: the caller knows the line's width from the buffer, and the
// suggestion's from the plain text it passed in. Escapes must never reach the
// column arithmetic, and the surest way to guarantee that is never to hand them
// anything that could be mistaken for a width.
func (t theme) paint(line string, cursor int, suggestion string, knows commandOracle) string {
	if !t.enabled {
		return line
	}
	painted := renderSpans(highlight(line, cursor, t.colours, knows))
	if suggestion != "" {
		painted += span{text: suggestion, codes: t.colours.suggestion}.render()
	}
	return painted
}

// suggests reports whether a suggestion may be drawn at all.
func (t theme) suggests() bool {
	return t.enabled && len(t.colours.suggestion) > 0
}
