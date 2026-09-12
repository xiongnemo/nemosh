package main

import "strings"

// Colouring a history reference by what it will become.
//
// `!!` was drawn **red**, and looked like a mistake every time it was typed. The reason
// is that the colour asks one question -- "can this shell run a command of this name?" --
// and `!!` is not a command name at all. It is a textual rewrite that happens before the
// parser sees anything, so no amount of looking in PATH for a program called `!!` will
// ever find one.
//
// Answering `undetermined` and drawing it plainly would fix the red, and would also throw
// away the one thing worth saying. `!!` is going to become the previous command, and the
// editor knows what that was, so it can be judged on what it will *run*:
//
//	ls -la        then  !!     -> green, because `ls` is runnable
//	nosuchprog    then  !!     -> red, because it still will not run
//	              then  !xyz   -> red, because the event does not exist
//
// That is the same question the colour always asked, asked of the right word.

// commandStanding answers for a word in command position, resolving a history reference
// first so the colour describes what will actually run.
func (e *lineEditor) commandStanding(word string) commandStanding {
	expanded, isReference := e.expandedCommandWord(word)
	if !isReference {
		return e.commands.standing(word)
	}
	if expanded == "" {
		// The reference resolves to nothing -- `!xyz` with no such event -- and the
		// line will be refused rather than run. Red is exactly right.
		return standingUnknown
	}
	return e.commands.standing(expanded)
}

// expandedCommandWord answers the command name a history reference will produce, and
// whether the word was a reference at all.
//
// Only the first word of the expansion is wanted: `!!` may bring back a whole pipeline,
// and what is being coloured is the command at the front of it.
func (e *lineEditor) expandedCommandWord(word string) (string, bool) {
	if !strings.ContainsAny(word, "!^") {
		// The overwhelmingly common case, and this runs on every keystroke for every
		// word, so it is answered before anything is allocated.
		return "", false
	}
	expanded, changed, err := expandHistory(word, e.history)
	if err != nil {
		// An unresolvable reference is still a reference; it just has no command.
		return "", true
	}
	if !changed {
		// A `!` that begins nothing -- `[ x != y ]`, or a bare `^` in a pattern -- is
		// ordinary text and must be judged as it was typed.
		return "", false
	}
	return strings.TrimSpace(firstWordOf(expanded)), true
}

// firstWordOf is the leading blank-separated word, which is the command position.
func firstWordOf(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	if end := strings.IndexAny(trimmed, " \t"); end >= 0 {
		return trimmed[:end]
	}
	return trimmed
}
