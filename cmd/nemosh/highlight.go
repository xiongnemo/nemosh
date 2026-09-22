package main

import (
	"strings"

	"github.com/xiongnemo/nemosh/internal/capability"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
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
	reservedWord   []string
	definition     []string
	knownOption    []string
	unknownOption  []string
	editingWord    []string
	suggestion     []string
}

func defaultPalette() palette {
	return palette{
		knownCommand:   []string{"32"}, // green
		unknownCommand: []string{"31"}, // red
		// A reserved word is not a command, and red -- this palette's "no such command" --
		// was an outright false claim about `case`. Magenta because it has to differ from
		// both verdicts beside it, and because a keyword is structure rather than a name.
		reservedWord: []string{"35"},
		// The `name` of `name()`: not runnable yet, and not unknown either. Blue and bold,
		// so a definition reads as a declaration rather than as a call.
		definition:    []string{"1", "34"},
		knownOption:   []string{"36"}, // cyan
		unknownOption: []string{"33"}, // yellow: unknown here is a guess, not a verdict
		editingWord:   []string{"4"},  // underline, and combined with whatever colour applies
		suggestion:    []string{"90"}, // bright black
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
func highlight(line string, cursor int, colours palette, knows commandOracle) []span {
	tokens := splitHighlightTokens(line)
	var spans []span
	commandPosition := true
	command := ""
	start, offset := 0, 0
	for index, token := range tokens {
		end := start + len([]rune(token.text))
		// A word where a case pattern goes is data. Without asking, the word after a `;;`
		// looked like a command position and `*)` was drawn as a command that does not
		// exist -- the same false claim `case` itself used to get. The grammar already
		// answers this question for the parser; see case_pattern_position.go.
		pattern := !token.blank && !token.operator && runtime.CasePatternPosition(line[:offset])
		switch {
		case token.blank:
			// Blanks carry no role and no cursor: underlining the gap you are typing into
			// would move the underline a character ahead of the word it is about.
			spans = append(spans, span{text: token.text})
		case token.operator:
			// Punctuation, drawn plainly -- the eye finds it without help. What it does
			// carry is the position: after `;;` or `)` a command begins again, which is
			// the whole of what the blank-splitting version got wrong.
			spans = append(spans, span{text: token.text, codes: cursorCodes(nil, colours, cursor, start, end)})
			commandPosition = runtime.CommandFollows(token.text)
		default:
			codes := wordCodes(token.text, command, commandPosition && !pattern, colours, knows,
				definesFunctionAt(tokens, index))
			if commandPosition && !pattern && !runtime.ReservedWord(token.text) {
				command = token.text
			}
			spans = append(spans, span{text: token.text, codes: cursorCodes(codes, colours, cursor, start, end)})
			commandPosition = runtime.CommandFollows(token.text)
		}
		start = end
		offset += len(token.text)
	}
	return spans
}

// cursorCodes adds the editing underline when the cursor is in this span.
//
// Anywhere inside it, including at its end, which is the common case: that is where the
// cursor is while a word is being typed.
func cursorCodes(codes []string, colours palette, cursor, start, end int) []string {
	if cursor < start || cursor > end {
		return codes
	}
	return append(codes, colours.editingWord...)
}

// definesFunctionAt reports whether the word at index is the name of a function being
// defined -- `name()`, or `name ()`, which is equally valid.
//
// Without this the name was drawn red: this palette's "no such command", said about a
// command that is being brought into existence on this very line.
func definesFunctionAt(tokens []highlightToken, index int) bool {
	for next := index + 1; next < len(tokens); next++ {
		if tokens[next].blank {
			continue
		}
		if tokens[next].text != "(" {
			return false
		}
		// `(` alone would be a subshell as an argument, which is not a thing; the pair is
		// what makes it a definition.
		for after := next + 1; after < len(tokens); after++ {
			if tokens[after].blank {
				continue
			}
			return tokens[after].text == ")"
		}
		return false
	}
	return false
}

// wordCodes decides how one word is drawn.
func wordCodes(word, command string, commandPosition bool, colours palette, knows commandOracle, defines bool) []string {
	// A reserved word before a verdict about commands, because it is not one. `case` was
	// drawn as a command that does not exist, which is not a missing colour but a false
	// statement -- and the grammar's own answer is used rather than a second list here.
	if runtime.ReservedWord(word) {
		return append([]string(nil), colours.reservedWord...)
	}
	if defines {
		return append([]string(nil), colours.definition...)
	}
	if commandPosition {
		switch knows(word) {
		case standingRunnable:
			return append([]string(nil), colours.knownCommand...)
		case standingUnknown:
			return append([]string(nil), colours.unknownCommand...)
		}
		// Undetermined: PATH has not been read yet, or the word names a file
		// rather than a command. Saying nothing is the honest colour.
		return nil
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

// Where a command begins is asked of runtime.CommandFollows now, and this is the note
// worth keeping about what that replaced.
//
// The rule used to be "a free-standing operator", so `ls | grep x` coloured grep and
// `ls|grep x` did not -- on the grounds that splitting the second correctly is parsing, and
// decoration may be approximate because being wrong is only ever a colour.
//
// That reasoning had a hole in it, and `bingo;;` is the hole: an operator glued to a word
// is not an unusual way to write a line, it is the *normal* way to write a case arm, and
// being wrong there silently turned the rest of the line into arguments. Splitting properly
// turned out to be a lexer rather than a parser -- see highlight_tokens.go -- and the same
// change let the reserved words be asked about too, which is what stopped `case` being
// drawn as a command that does not exist.
