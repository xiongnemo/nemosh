package applets

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Search and replace, asking about each match.
//
// `-H` listed "No replace; search only" from the day the editor was written, and the
// reason given was real: replace needs a *second* kind of prompt. The existing one
// collects a line and fires on Enter, which is right for "what shall I search for" and
// wrong for "shall I replace this one" -- that has to answer to a single key, because
// pressing Enter after every `y` through forty matches is not a feature.
//
// So this adds one: `confirm`, a handler for one keystroke, checked before the line
// prompt. Everything else is built from what was already there.
//
// **The scan starts at the top of the buffer, not at the cursor.** nano starts at the
// cursor and wraps. Starting at the top is the answer to "fix every occurrence in this
// file", which is what replace is nearly always for, and it has no wrap condition to get
// wrong -- no question of whether the run has come back round to where it began, and no
// way to be left wondering whether some were missed. The divergence is in `-H`.

// editorReplaceState is one replace run: what to look for, what to put there, and how far
// the scan has got.
type editorReplaceState struct {
	needle string
	with   string
	// line and column are where the match being offered starts. The column is a byte
	// offset into the line, which is what strings.Index answers and what the splice
	// below needs; it is never shown to anyone.
	line     int
	column   int
	replaced int
}

// askReplace runs the two prompts, then starts the confirmation loop.
func (v *editorView) askReplace() {
	if v.session.readOnly {
		v.setMessage("[red]Read-only: the buffer cannot be changed[-]")
		return
	}
	v.ask("Replace: ", func(needle string) {
		if needle == "" {
			v.setMessage("Cancelled")
			return
		}
		// Asked for even when empty: replacing with nothing is deleting every
		// occurrence, which is a thing people want and not a mistake to guard against.
		v.ask("Replace with: ", func(with string) {
			v.replace = &editorReplaceState{needle: needle, with: with}
			v.offerReplacement()
		})
	})
}

// offerReplacement moves to the next match and asks about it, or ends the run.
func (v *editorView) offerReplacement() {
	state := v.replace
	if state == nil {
		return
	}
	lines, _ := v.lines()
	line, column, found := findEditorMatch(lines, state.needle, state.line, state.column)
	if !found {
		v.finishReplace()
		return
	}
	state.line, state.column = line, column
	v.moveToLine(line)
	v.confirm = v.answerReplacement
	v.setMessage("Replace on line %d? [yellow]y[-]es  [yellow]n[-]o  [yellow]a[-]ll  [yellow]q[-]uit", line+1)
}

// answerReplacement handles one keystroke of the confirmation.
func (v *editorView) answerReplacement(answer rune) {
	state := v.replace
	if state == nil {
		return
	}
	switch answer {
	case 'y':
		v.applyReplacement()
		v.offerReplacement()
	case 'n':
		// Past this match rather than past this line: two occurrences on one line are
		// two separate questions.
		state.column += len(state.needle)
		v.offerReplacement()
	case 'a':
		for {
			lines, _ := v.lines()
			line, column, found := findEditorMatch(lines, state.needle, state.line, state.column)
			if !found {
				break
			}
			state.line, state.column = line, column
			v.applyReplacement()
		}
		v.finishReplace()
	case 'q':
		v.finishReplace()
	default:
		// An unrecognised key asks again rather than guessing. Guessing `n` would be
		// safe and guessing `y` would not, and a prompt that silently treats every
		// stray key as "no" is one people learn to distrust.
		v.confirm = v.answerReplacement
		v.setMessage("Replace on line %d? [yellow]y[-]es  [yellow]n[-]o  [yellow]a[-]ll  [yellow]q[-]uit", state.line+1)
	}
}

// applyReplacement splices one occurrence and steps past what it wrote.
//
// Past the *replacement*, not past the needle: replacing `a` with `aa` would otherwise
// find the second `a` it had just written and do it again, for as long as the buffer held
// out.
func (v *editorView) applyReplacement() {
	state := v.replace
	lines, _ := v.lines()
	if state.line >= len(lines) {
		return
	}
	line := lines[state.line]
	if state.column < 0 || state.column+len(state.needle) > len(line) {
		return
	}
	lines[state.line] = line[:state.column] + state.with + line[state.column+len(state.needle):]
	v.setLines(lines, state.line)
	state.column += len(state.with)
	state.replaced++
}

// finishReplace ends the run and reports what it did.
func (v *editorView) finishReplace() {
	state := v.replace
	v.replace, v.confirm = nil, nil
	if state == nil {
		return
	}
	switch state.replaced {
	case 0:
		v.setMessage("[yellow]%q not found[-]", state.needle)
	case 1:
		v.setMessage("Replaced one occurrence")
	default:
		v.setMessage("Replaced %d occurrences", state.replaced)
	}
}

// findEditorMatch is the next occurrence at or after a position.
func findEditorMatch(lines []string, needle string, fromLine, fromColumn int) (int, int, bool) {
	if needle == "" {
		return 0, 0, false
	}
	for line := max(fromLine, 0); line < len(lines); line++ {
		start := 0
		if line == fromLine {
			start = fromColumn
		}
		if start > len(lines[line]) {
			continue
		}
		if index := strings.Index(lines[line][start:], needle); index >= 0 {
			return line, start + index, true
		}
	}
	return 0, 0, false
}

// handleConfirmKey answers a single-key prompt.
//
// Escape is `q`: leaving by the key that cancels everything else here should stop the run
// rather than do nothing, and stopping keeps whatever has already been replaced -- which
// is what `q` means in the reference too.
func (v *editorView) handleConfirmKey(event *tcell.EventKey) *tcell.EventKey {
	handler := v.confirm
	v.confirm = nil
	switch {
	case event.Key() == tcell.KeyEscape:
		handler('q')
	case event.Key() == tcell.KeyRune:
		// Lower-cased, so a stray Caps Lock does not turn every answer into an
		// unrecognised key.
		handler([]rune(strings.ToLower(string(event.Rune())))[0])
	default:
		handler(0)
	}
	return nil
}
