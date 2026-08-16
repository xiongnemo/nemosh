package main

import (
	"strings"

	"github.com/xiongnemo/nemosh/internal/completionspec"
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
	// commands is every name that can be run: builtins, applets, this session's
	// aliases and functions, and everything on PATH.
	commands []string
	// hosts is the machines ~/.ssh/config names. In memory like everything else
	// here, which is the whole reason the index behind it exists.
	hosts []string
	// specs answers what a command's operand is, so the grey text and the Tab
	// key cannot disagree about what the word being typed is.
	specs *completionspec.Registry
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
	if remainder, ok := s.fromHostNames(line); ok {
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
	typed := len([]rune(line))
	best := ""
	for _, name := range s.commands {
		// completionMatches, not HasPrefix: Tab and the suggestion are looking at
		// one list and have to agree about what matches in it. They did not, and
		// on Windows that showed as `WH` finding eight commands under Tab and
		// suggesting nothing.
		if !completionMatches(name, line) || len([]rune(name)) == typed {
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
	// Sliced by runes, not bytes: the match may be a different case from what was
	// typed, and folding can change a rune's byte length without changing the
	// count. Byte arithmetic against the typed text would cut the name mid-rune.
	return string([]rune(best)[typed:]), true
}

// fromHostNames guesses the operand of `ssh`, which is the one operand in this
// shell that comes from a closed set rather than from the filesystem.
//
// This is the idea zsh-autosuggestions calls its `completion` strategy: ask what
// completion would offer and show the first answer, so a suggestion can appear
// for something never typed before. It is restricted here to the one source that
// is already in memory, because the rule that nothing on this path touches the
// filesystem is what keeps typing from stuttering.
//
// Only when completion would offer a host for this exact word. That check is the
// same operandTargetFor the Tab key uses, so the grey text and the Tab key can
// never disagree about what the word is -- which is the failure the case-folding
// bug already demonstrated once.
func (s suggester) fromHostNames(line string) (string, bool) {
	word := line[len(line)-len(currentSuggestionWord(line)):]
	if operandTargetFor(s.specs, line[:len(line)-len(word)]) != targetHost {
		return "", false
	}
	name := word
	if at := strings.LastIndexByte(word, '@'); at >= 0 {
		name = word[at+1:]
	}
	if name == "" {
		return "", false
	}
	best := ""
	for _, host := range s.hosts {
		if !completionMatches(host, name) || len([]rune(host)) == len([]rune(name)) {
			continue
		}
		// The shortest, as with command names: the least the shell is
		// committing to on the strength of a guess.
		if best == "" || len(host) < len(best) {
			best = host
		}
	}
	if best == "" {
		return "", false
	}
	return string([]rune(best)[len([]rune(name)):]), true
}

// currentSuggestionWord is the unfinished word at the end of the line. Blanks
// only: a suggestion is offered for what is being typed, and the operators that
// would separate commands are already handled by the callers above.
func currentSuggestionWord(line string) string {
	if start := strings.LastIndexAny(line, " \t"); start >= 0 {
		return line[start+1:]
	}
	return line
}
