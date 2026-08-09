package main

import (
	"strings"

	"github.com/xiongnemo/nemosh/internal/capability"
)

// Drawing the line in colour is decoration, and decoration must not lie. Three
// rules follow from that and shape everything here.
//
// It never changes what is in the buffer. Every escape is added at draw time, so
// the submitted line is exactly what was typed and the column arithmetic that
// places the cursor never sees an escape at all.
//
// It never guesses harder than it knows. A word is coloured by whether *this
// shell* carries a command of that name; an external program on PATH is not in
// that table, so unknown is drawn as a warning rather than an error, and the
// suggestion engine says the same thing in the same place.
//
// And if colour cannot be shown it draws nothing, rather than falling back to
// plain text. A grey suggestion that renders as ordinary text is worse than no
// suggestion: it puts characters on screen that are not in the line.

// palette is every colour choice in one place, deliberately. Changing how a
// known command looks should be one edit, not a search.
//
// The values are SGR parameter numbers, joined with `;` into one escape. Empty
// means draw that role plainly.
type palette struct {
	knownCommand   []string
	unknownCommand []string
	knownOption    []string
	unknownOption  []string
	editingWord    []string
	suggestion     []string
}

func defaultPalette() palette {
	return palette{
		knownCommand:   []string{"32"}, // green
		unknownCommand: []string{"31"}, // red
		knownOption:    []string{"36"}, // cyan
		unknownOption:  []string{"33"}, // yellow: unknown here is a guess, not a verdict
		editingWord:    []string{"4"},  // underline, and combined with whatever colour applies
		suggestion:     []string{"90"}, // bright black
	}
}

// span is a run of text drawn with one set of attributes.
type span struct {
	text  string
	codes []string
}

func (s span) render() string {
	if len(s.codes) == 0 || s.text == "" {
		return s.text
	}
	return "\033[" + strings.Join(s.codes, ";") + "m" + s.text + "\033[0m"
}

func renderSpans(spans []span) string {
	var out strings.Builder
	for _, s := range spans {
		out.WriteString(s.render())
	}
	return out.String()
}

// highlight turns the edited line into spans.
//
// cursor is a rune index, and it decides one thing only: which word is being
// edited. That word is underlined while it is still being typed -- until a blank
// ends it -- which is the visible answer to "what will Tab act on".
func highlight(line string, cursor int, colours palette) []span {
	runes := []rune(line)
	var spans []span
	commandPosition := true
	command := ""
	index := 0
	for index < len(runes) {
		if runes[index] == ' ' {
			start := index
			for index < len(runes) && runes[index] == ' ' {
				index++
			}
			spans = append(spans, span{text: string(runes[start:index])})
			continue
		}
		start := index
		for index < len(runes) && runes[index] != ' ' {
			if runes[index] == '\\' && index+1 < len(runes) {
				index++
			}
			index++
		}
		if index > len(runes) {
			index = len(runes)
		}
		word := string(runes[start:index])
		codes := wordCodes(word, command, commandPosition, colours)
		if commandPosition && !isCommandSeparatorWord(word) {
			command = word
		}
		// The cursor sitting anywhere inside the word, including at its end,
		// means this is the one being edited. At its end is the common case --
		// that is where the cursor is while you type.
		if cursor >= start && cursor <= index {
			codes = append(codes, colours.editingWord...)
		}
		spans = append(spans, span{text: word, codes: codes})
		commandPosition = isCommandSeparatorWord(word)
	}
	return spans
}

// wordCodes decides how one word is drawn.
func wordCodes(word, command string, commandPosition bool, colours palette) []string {
	if isCommandSeparatorWord(word) {
		return nil
	}
	if commandPosition {
		if knownCommand(word) {
			return append([]string(nil), colours.knownCommand...)
		}
		return append([]string(nil), colours.unknownCommand...)
	}
	switch optionStanding(command, word) {
	case optionAccepted:
		return append([]string(nil), colours.knownOption...)
	case optionUnknown:
		return append([]string(nil), colours.unknownOption...)
	}
	return nil
}

type optionVerdict int

const (
	// notAnOption covers an operand, and also every word belonging to a command
	// this shell knows nothing about -- an external program's options are not in
	// the table, and colouring them by its absence would be inventing a verdict.
	notAnOption optionVerdict = iota
	optionAccepted
	optionUnknown
)

// optionStanding asks the capability table whether a command takes an option.
//
// The same table completion offers from, so the two can never disagree: an
// option Tab offers is an option drawn as accepted, because there is one place
// that says so and a test holds it against what the applets really do.
func optionStanding(command, word string) optionVerdict {
	if command == "" || len(word) < 2 || word[0] != '-' {
		return notAnOption
	}
	entry, ok := capability.Lookup(command)
	if !ok {
		return notAnOption
	}
	if word == "--" {
		// End of options, not an option.
		return notAnOption
	}
	if strings.HasPrefix(word, "--") {
		name, _, _ := strings.Cut(word[2:], "=")
		if entry.AcceptsLong(name) {
			return optionAccepted
		}
		return optionUnknown
	}
	// A short cluster is accepted only if every letter in it is: `ls -al` is two
	// options and `ls -aZ` is one option and a mistake, so the whole word is
	// drawn as the mistake.
	for _, flag := range word[1:] {
		if !entry.AcceptsShort(flag) {
			return optionUnknown
		}
	}
	return optionAccepted
}

// isCommandSeparatorWord reports whether a word is one of the operators that
// starts a new command, so that the word after it is coloured as a command name
// rather than as an operand.
//
// Only a free-standing operator is recognised. `ls | grep x` colours grep;
// `ls|grep x` does not, because splitting that correctly is parsing, and this is
// decoration -- being approximate is acceptable where being wrong is only ever a
// colour. The rule is the same one completesCommand uses, kept deliberately in
// step with it.
func isCommandSeparatorWord(word string) bool {
	switch word {
	case "|", "||", "&", "&&", ";", ";;", "(", "{":
		return true
	}
	return false
}
