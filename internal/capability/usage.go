package capability

import (
	"fmt"
	"sort"
	"strings"
)

// Usage text for the applets, rendered from the same rows that describe what they
// accept.
//
// Not one hand-written string per applet, which is what would rot. The option
// letters in the usage line come from the matrix -- already measured against a
// built binary and already bound to behaviour by capability_test.go -- so a letter
// cannot appear in the help without the applet taking it, and cannot be taken
// without appearing. What is written by hand is only what the matrix cannot know:
// a sentence about what the applet is for, a gloss per option, and the shape of
// the operands. usage_test.go binds those too: a documented option that does not
// exist and an existing option with no gloss both fail.
//
// The gap this closes was stated outright in the shell's own `help` builtin, which
// used to end with "Applets carry no usage text". Every applet rejected `--help`,
// which is the first thing anyone types.

// Usage is what one applet's help says beyond its option letters.
type Usage struct {
	// Summary is one line, in the imperative, about what the applet does.
	Summary string
	// Operands is the operand part of the usage line, in the conventional
	// notation: `[FILE]...` for many optional files, `SOURCE... DEST` for a copy.
	// Empty for an applet that takes none.
	Operands string
	// Options glosses each accepted option, keyed by the letter for a short one
	// and by the name for a long one. A few words, lower case, no full stop.
	Options map[string]string
	// Notes are lines appended after the options, for the things a reader would
	// otherwise have to discover. Where this build knowingly differs from GNU or
	// busybox, this is where it says so.
	Notes []string
}

// UsageFor renders the help for one command, and reports whether there is any.
func UsageFor(name string) (string, bool) {
	command, ok := Lookup(name)
	if !ok {
		return "", false
	}
	usage, ok := usageText[name]
	if !ok {
		return "", false
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Usage: %s\n", usageLine(command, usage))
	fmt.Fprintf(&out, "\n%s\n", usage.Summary)
	if options := renderOptions(command, usage); options != "" {
		fmt.Fprintf(&out, "\nOptions:\n%s", options)
	}
	// One blank line before the notes, then one line each.
	//
	// A note is a line of prose rather than a paragraph, which is how all three applets with
	// more than one are written: ps and top both split a sentence across two entries. A blank
	// line between every note put a paragraph break in the middle of a sentence, and top's help
	// -- the longest of them -- read as a list of disconnected fragments.
	if len(usage.Notes) > 0 {
		out.WriteString("\n")
		for _, note := range usage.Notes {
			fmt.Fprintf(&out, "%s\n", note)
		}
	}
	return out.String(), true
}

// usageLine is the synopsis: the name, a bracket for the clustered short options,
// then each option that takes a value, then the operands.
func usageLine(command Command, usage Usage) string {
	parts := []string{command.Name}
	if flags := flagOnlyLetters(command); flags != "" {
		parts = append(parts, "[-"+flags+"]")
	}
	for _, letter := range sortedLetters(command.ValueShort) {
		parts = append(parts, fmt.Sprintf("[-%s %s]", letter, valuePlaceholder(command.Name, letter)))
	}
	for _, name := range command.Long {
		parts = append(parts, "[--"+name+"]")
	}
	if usage.Operands != "" {
		parts = append(parts, usage.Operands)
	}
	return strings.Join(parts, " ")
}

// flagOnlyLetters are the short options that stand alone, so they can be shown
// clustered the way they are typed.
func flagOnlyLetters(command Command) string {
	var flags strings.Builder
	for _, letter := range sortedLetters(command.Short) {
		if strings.Contains(command.ValueShort, letter) {
			continue
		}
		flags.WriteString(letter)
	}
	return flags.String()
}

func sortedLetters(letters string) []string {
	out := make([]string, 0, len(letters))
	for _, letter := range letters {
		out = append(out, string(letter))
	}
	sort.Strings(out)
	return out
}

func renderOptions(command Command, usage Usage) string {
	var out strings.Builder
	for _, letter := range sortedLetters(command.Short) {
		spelling := "-" + letter
		if strings.Contains(command.ValueShort, letter) {
			spelling += " " + valuePlaceholder(command.Name, letter)
		}
		fmt.Fprintf(&out, "  %-14s %s\n", spelling, usage.Options[letter])
	}
	for _, name := range command.Long {
		fmt.Fprintf(&out, "  %-14s %s\n", "--"+name, usage.Options[name])
	}
	return out.String()
}

// valuePlaceholder names what an option's argument is, so `-k KEYDEF` reads as
// something rather than as `-k VALUE`. Falls back to VALUE where nothing better
// has been recorded, which is honest rather than wrong.
func valuePlaceholder(command, letter string) string {
	if placeholder, ok := valuePlaceholders[command+letter]; ok {
		return placeholder
	}
	return "VALUE"
}
