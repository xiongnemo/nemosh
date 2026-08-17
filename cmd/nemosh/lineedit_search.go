package main

import (
	"fmt"
	"strings"
)

// Ctrl-R: incremental reverse search through history.
//
// The reflex every other shell has, and the one that was conspicuously missing
// once history started surviving the window. `history` prints and the arrows
// walk, but neither answers "the docker command from Tuesday" -- and a list you
// have to read is not the same tool as a search you type into.
//
// readline's behaviour is the one people's fingers know, and this follows it:
//
//   - The prompt becomes `(reverse-i-search)'pattern': line`, bash's exact
//     shape, so the mode is unmistakable rather than a line that mysteriously
//     changed.
//   - Each character narrows, searching backwards from where the last match was.
//   - Ctrl-R again steps to the next older match; at the oldest it stops rather
//     than wrapping, and says `failed` the way readline does.
//   - Enter accepts the match onto the line -- and, unlike readline, does not
//     run it. That is deliberate; see below.
//   - Ctrl-C or Ctrl-G abandons, restoring the line exactly as it was.
//   - Any editing key -- an arrow, Home, Backspace on an empty pattern --
//     accepts the match and leaves the mode, which is how readline lets you
//     land on a line and then fix it.
//
// The one divergence is Enter. readline runs the line immediately, which is
// famously how people execute the wrong `rm`: the match is chosen by a substring
// nobody re-reads before the newline. Here Enter puts the match on the line and
// leaves the cursor at its end, so it is submitted by a second, deliberate Enter.
// zsh-autosuggestions and fish's history-search behave this way, and it costs one
// keystroke to make the destructive case visible.
type historySearch struct {
	// pattern is what has been typed into the search.
	pattern string
	// index is the history entry currently matched, or -1 for none.
	index int
	// restore is the line as it was when the search began, put back if the
	// search is abandoned.
	restore string
	// failed records that the last narrowing found nothing, which the prompt
	// says out loud.
	failed bool
}

// searchPrompt is what replaces the ordinary prompt while searching. bash's
// wording, including the quotes and the trailing colon.
func (s historySearch) searchPrompt() string {
	if s.failed {
		return fmt.Sprintf("(failed reverse-i-search)`%s': ", s.pattern)
	}
	return fmt.Sprintf("(reverse-i-search)`%s': ", s.pattern)
}

// beginHistorySearch enters the mode, remembering the line to restore.
func (e *lineEditor) beginHistorySearch() {
	e.search = &historySearch{index: len(e.history), restore: e.buffer.String()}
}

// searchOlder looks backwards from the entry before `from` for one containing the
// pattern, and reports whether it found one.
//
// A substring rather than a prefix, which is the whole point: the useful memory
// of a command is rarely its first word. Case-sensitive, as readline is by
// default -- a history line is text somebody typed, and folding it would make
// `Dockerfile` and `dockerfile` the same entry.
func (e *lineEditor) searchOlder(pattern string, from int) (int, bool) {
	if pattern == "" {
		return from, false
	}
	for index := from - 1; index >= 0; index-- {
		if strings.Contains(e.history[index], pattern) {
			return index, true
		}
	}
	return from, false
}

// narrowSearch adds a character to the pattern.
func (e *lineEditor) narrowSearch(r rune) {
	pattern := e.search.pattern + string(r)
	// From the current match *inclusive*, so typing another character of a
	// pattern the current line still satisfies keeps you on that line rather
	// than jumping past it. readline does the same.
	from := e.search.index + 1
	if from > len(e.history) {
		from = len(e.history)
	}
	index, found := e.searchOlder(pattern, from)
	e.search.pattern = pattern
	e.search.failed = !found
	if found {
		e.search.index = index
		e.buffer.replace(e.history[index])
	}
}

// stepSearch is a second Ctrl-R: the next older match.
func (e *lineEditor) stepSearch() {
	index, found := e.searchOlder(e.search.pattern, e.search.index)
	e.search.failed = !found
	if found {
		e.search.index = index
		e.buffer.replace(e.history[index])
	}
}

// widenSearch is Backspace: one character off the pattern, and the match
// recomputed from the newest entry so the search can go forwards again.
func (e *lineEditor) widenSearch() {
	if e.search.pattern == "" {
		// Nothing left to remove. readline leaves the mode here rather than
		// beeping, which is also the least surprising thing: the pattern is
		// empty, so there is nothing being searched for.
		e.endHistorySearch(true)
		return
	}
	runes := []rune(e.search.pattern)
	e.search.pattern = string(runes[:len(runes)-1])
	index, found := e.searchOlder(e.search.pattern, len(e.history))
	e.search.failed = e.search.pattern != "" && !found
	if found {
		e.search.index = index
		e.buffer.replace(e.history[index])
	}
}

// endHistorySearch leaves the mode, keeping the matched line or restoring the
// original.
func (e *lineEditor) endHistorySearch(accept bool) {
	if !accept {
		e.buffer.replace(e.search.restore)
	}
	e.search = nil
	e.buffer.moveEnd()
	// The prompt is about to change back, and the search prompt was a different
	// width. Nothing of the old drawing can be trusted.
	e.resetDrawState()
}

// searching reports whether the mode is active, safe on a nil search.
func (e *lineEditor) searching() bool { return e.search != nil }

// searchKey handles one key while the search is up, reporting whether it consumed
// it.
//
// The mode is modal on purpose: while it is up, an ordinary letter narrows the
// pattern instead of being inserted. Anything that is neither a letter nor one of
// the search's own keys accepts the match, leaves the mode, and is *not*
// consumed -- the caller then handles it normally, in the same keystroke. That is
// how readline lets you land on a line with Ctrl-R and fix it with an arrow
// without the arrow being swallowed.
func (e *lineEditor) searchKey(pressed key) bool {
	switch pressed.kind {
	case keyRune:
		e.narrowSearch(pressed.value)
	case keyReverseSearch:
		e.stepSearch()
	case keyBackspace:
		e.widenSearch()
	case keyEnter:
		// Accepted onto the line, not run. See the note on historySearch: the
		// match was chosen by a substring, and the destructive case deserves to
		// be looked at before a newline reaches it.
		e.endHistorySearch(true)
	case keyInterrupt, keyAbort:
		// Ctrl-C and Ctrl-G both abandon, putting the original line back. Note
		// that Ctrl-C here does not abandon the whole line as it does outside the
		// search -- it undoes the search, which is readline's behaviour and
		// leaves the least work to redo.
		e.endHistorySearch(false)
	case keyEndOfInput:
		// Ctrl-D inside a mode would otherwise end the session, which is a
		// surprising way to lose a line. Treated as leaving the search with what
		// was found.
		e.endHistorySearch(true)
	default:
		e.endHistorySearch(true)
		return false
	}
	return true
}

// drawSearch repaints the line under the search's own prompt.
//
// The prompt changes width with every keystroke -- `(reverse-i-search)` grows by
// one character each time -- so the previous draw's geometry is worthless and the
// row is cleared rather than patched. This is the one place the prompt is not
// constant for the life of the line.
func (e *lineEditor) drawSearch() {
	fmt.Fprint(e.screen, "\r\033[K")
	e.resetDrawState()
	prompt := e.search.searchPrompt()
	fmt.Fprint(e.screen, prompt)
	e.drawn, e.drawnRows = 0, 0
	e.redraw(prompt)
}

// restorePrompt puts the ordinary prompt back after the search's own, which was
// a different width. The row is cleared rather than patched for the same reason
// drawSearch clears it: none of the previous geometry describes what is there.
func (e *lineEditor) restorePrompt(prompt string) {
	fmt.Fprint(e.screen, "\r\033[K")
	e.resetDrawState()
	fmt.Fprint(e.screen, prompt)
}
