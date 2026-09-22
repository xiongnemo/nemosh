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
	prefix := string(e.buffer.runes[:e.buffer.cursor])
	target, found := e.matchingHistory(prefix, direction)
	if !found {
		return
	}
	e.recall = target
	if target == 0 {
		// Back at the line that was being typed. Only the prefix is known -- the rest was
		// replaced by a recalled entry -- and the prefix is what was typed, so it goes
		// back rather than an empty line.
		e.buffer.replace(prefix)
		return
	}
	e.buffer.replace(e.history[len(e.history)-target])
	// The cursor returns to the end of the prefix rather than the end of the line, which
	// is what keeps the next press searching for the same thing.
	if runes := len([]rune(prefix)); runes <= e.buffer.length() {
		e.buffer.cursor = runes
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
