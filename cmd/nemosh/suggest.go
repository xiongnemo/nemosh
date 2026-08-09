package main

import (
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/capability"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// A suggestion is what the line would most likely become, drawn ahead of the
// cursor in grey and accepted only if asked for.
//
// It is a different question from completion, and the difference decides the
// design. Completion answers "what could go here" when asked, and may take as
// long as reading a directory. A suggestion answers "what did you probably mean"
// after *every keystroke*, and shows one answer speculatively -- so every source
// it consults has to be in memory. Nothing here touches the filesystem, and that
// is a rule rather than an accident: a directory read per keystroke is felt on a
// network drive, and a shell that stutters as you type is worse than one that
// suggests nothing.
//
// The sources are tried in order, which is the shape zsh gives its completers:
// each is a different guess, and the first that answers wins.
type suggester struct {
	// history is the lines already run, oldest first.
	history []string
}

// suggest returns the text to draw after the line, or "" for none.
//
// The returned text is the *remainder*: what would be added, not the whole line.
// Returning the remainder rather than the completed line is what keeps the
// renderer honest -- it cannot accidentally redraw what the user typed in a
// different colour, because it never has it.
func (s suggester) suggest(line string) string {
	if line == "" || strings.HasSuffix(line, " ") {
		// Nothing has been typed toward the next word yet. Guessing here would
		// mean guessing a whole word from nothing, which fish does not do either.
		return ""
	}
	if remainder, ok := s.fromHistory(line); ok {
		return remainder
	}
	if remainder, ok := s.fromCommandNames(line); ok {
		return remainder
	}
	return ""
}

// fromHistory is the strongest guess and the one fish leads with: a line already
// run is a line meant. Most recent first, because a repeated command is usually
// the one just run.
func (s suggester) fromHistory(line string) (string, bool) {
	for index := len(s.history) - 1; index >= 0; index-- {
		entry := s.history[index]
		if len(entry) > len(line) && strings.HasPrefix(entry, line) {
			return entry[len(line):], true
		}
	}
	return "", false
}

// fromCommandNames guesses the first word, which is what history cannot help
// with in a fresh session -- and a fresh session is exactly where a suggestion
// engine looks broken if it has nothing to say.
//
// Only the first word: a name is a closed set this shell knows, while an operand
// is a filesystem question and belongs to completion.
func (s suggester) fromCommandNames(line string) (string, bool) {
	if strings.ContainsAny(line, " \t|&;<>()") {
		return "", false
	}
	best := ""
	for _, name := range commandNames() {
		if !strings.HasPrefix(name, line) || len(name) == len(line) {
			continue
		}
		// The shortest match, so the suggestion is the least the shell is
		// committing to. `e` suggesting `echo` is useful; `e` suggesting
		// `exit` merely because it sorts first is not.
		if best == "" || len(name) < len(best) {
			best = name
		}
	}
	if best == "" {
		return "", false
	}
	return best[len(line):], true
}

// commandNames is every name this shell can run without consulting PATH. Built
// once: it cannot change during a session, and this is on the keystroke path.
var commandNames = func() func() []string {
	var names []string
	return func() []string {
		if names != nil {
			return names
		}
		names = append(names, runtime.BuiltinNames()...)
		names = append(names, applets.DefaultRegistry.Names()...)
		return names
	}
}()

// knownCommand reports whether a command word names something this shell can
// run, which is what decides the colour it is drawn in.
//
// An unknown name is not necessarily wrong -- it may be a program on PATH, or a
// function or alias this session defined -- so the caller decides how loud to be
// about it. What this answers is only whether the shell itself carries it.
func knownCommand(name string) bool {
	return capability.Known(name)
}
