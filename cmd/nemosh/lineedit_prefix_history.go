package main

import "strings"

// History filtered by what you have already typed: zsh's
// history-beginning-search-backward, on Page Up.
//
// There were two ways to reach history and a gap between them. The arrows walk everything
// in order, which is what you want when the command was recent. Ctrl-R searches anywhere in
// the line, incrementally, which is what you want when you remember a word from the middle.
// Neither answers the commonest question -- "the last thing I ran that *started* like
// this" -- and that is how a command is usually remembered, because it is how it was
// thought of when it was typed.
//
// **The prefix is what lies left of the cursor**, not the whole line. That is what makes
// the key repeatable: the cursor stays where it was, so a second press narrows from the
// same prefix rather than from whatever was just recalled.
//
// An empty prefix makes it the arrows, which is what zsh does. Pressing it before typing
// anything should do something ordinary rather than nothing.
//
// Not in busybox's editor, which binds Page Up to nothing at all, so this is an extension
// rather than parity and docs/support-matrix.md records it as one.

// searchHistoryByPrefix moves through the entries that begin with the text left of the
// cursor. A positive direction goes back in time, which is the direction Page Up means.
//
// The line is left untouched when nothing matches. A key that cleared what you had typed
// because it could not find it would be worse than one that did nothing.
func (e *lineEditor) searchHistoryByPrefix(direction int) {
	// The prefix comes from the line that was *typed*, saved when the walk began -- never
	// from the buffer, which after Up or a previous press holds a history entry. Taking it
	// from the buffer made the prefix a whole recalled line, so Page Up after Up found
	// nothing; and handed that recalled text back at the end as though it had been typed.
	e.beginHistoryWalk()
	prefix := string([]rune(e.typed)[:e.typedCursor])
	target, found := e.matchingHistory(prefix, direction)
	if !found {
		return
	}
	e.showHistory(target)
	// Back at the typed line, the cursor is already where it was left. On a match it goes
	// to the end of the prefix, which is what keeps the next press searching for the same
	// thing -- unless there is no prefix, where that would be column zero and the next
	// thing typed would land in front of the recalled line. Up leaves the cursor at the
	// end, and an empty prefix is Up.
	if target != 0 && prefix != "" {
		e.buffer.cursor = len([]rune(prefix))
	}
}

// matchingHistory is the index of the next entry in that direction whose text begins with
// prefix, counted from the end the way recall is.
//
// Zero is the line being typed and is always a valid destination going forward: it is where
// Page Down ends up, and it always "matches" because the prefix came from it.
func (e *lineEditor) matchingHistory(prefix string, direction int) (int, bool) {
	for target := e.recall + direction; target >= 0 && target <= len(e.history); target += direction {
		if target == 0 {
			return 0, true
		}
		if strings.HasPrefix(e.history[len(e.history)-target], prefix) {
			return target, true
		}
	}
	return 0, false
}
