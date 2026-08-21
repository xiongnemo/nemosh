package applets

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// What a key press does, and the two things it can put on the screen instead of the table.
//
// Separate from the drawing because they are separate concerns and because top_view.go reached the
// file-size ceiling, which is the usual reason a file here splits. tview's own table keys -- the
// arrows, page up and down, home and end -- are not handled at all: returning the event leaves them
// to the widget, which is why there is no movement code anywhere in this package.

// key folds a press into the model and does what it asks.
//
// tview's own table keys -- the arrows, page up and down, home and end -- are left alone by
// returning the event, which is why there is no movement code here. What this handles is
// everything a monitor adds on top.
func (v *topView) key(event *tcell.EventKey) *tcell.EventKey {
	if v.prompting {
		// Something else owns the keyboard -- a prompt, or the kill confirmation. This
		// capture is on the *application*, so it sees every key before any widget does, and
		// without this a filter typed into an input field was read as commands: the `q` in
		// `sqlservr` quit the program.
		return event
	}
	name := topKeyName(event)
	if name == "" {
		return event
	}
	switch v.session.model.applyKey(name) {
	case topActionQuit:
		v.application.Stop()
	case topActionRefresh:
		v.refresh()
	case topActionKill:
		v.confirmKill()
	case topActionLowerPriority:
		v.adjustPriority(-1)
	case topActionRaisePriority:
		v.adjustPriority(1)
	case topActionHelp:
		v.status.SetText(topHelpText)
	case topActionFilterPrompt:
		v.promptFilter()
	case topActionSearchPrompt:
		v.promptSearch()
	case topActionSearchNext:
		v.searchNext()
	default:
		// The model absorbed it -- a sort, a toggle -- so redraw with the new
		// arrangement rather than waiting for the next tick.
		v.rememberSelection()
		v.refresh()
	}
	return nil
}

// topKeyName is the model's vocabulary for a key press, so the model never sees a tcell type.
func topKeyName(event *tcell.EventKey) string {
	switch event.Key() {
	case tcell.KeyEscape:
		return "esc"
	case tcell.KeyF1:
		return "F1"
	case tcell.KeyF3:
		return "F3"
	case tcell.KeyF5:
		return "F5"
	case tcell.KeyF7:
		return "F7"
	case tcell.KeyF8:
		return "F8"
	case tcell.KeyF4:
		return "F4"
	case tcell.KeyF9:
		return "F9"
	case tcell.KeyRune:
		if event.Rune() == ' ' {
			return "space"
		}
		return string(event.Rune())
	}
	return ""
}

// rememberSelection records which process the cursor is on, so a redraw can put it back.
func (v *topView) rememberSelection() {
	row, _ := v.table.GetSelection()
	if row >= 1 && row-1 < len(v.rows) {
		v.session.model.Selected = v.rows[row-1].Process.PID
	}
}

// confirmKill asks before terminating, because on Windows there is no gentler option.
//
// TerminateProcess gives the target no chance to close a handle or remove a lock file -- the same
// thing `kill` in this shell already documents. A monitor that did that on one keypress would be
// a trap, so the modal names the process.
func (v *topView) confirmKill() {
	v.rememberSelection()
	row, found := v.selectedRow()
	if !found {
		return
	}
	message := fmt.Sprintf("Terminate %s (pid %d)?\n\nWindows has no gentle signal: the process gets\nno chance to close handles or remove lock files.",
		row.Process.Name, row.Process.PID)
	modal := tview.NewModal().SetText(message).AddButtons([]string{"Cancel", "Terminate"})
	modal.SetDoneFunc(func(index int, label string) {
		v.prompting = false
		if label == "Terminate" {
			if err := proc.Terminate(row.Process.PID, 15); err != nil {
				v.status.SetText(fmt.Sprintf("[red]kill %d: %v", row.Process.PID, err))
			} else {
				v.status.SetText(fmt.Sprintf("[green]terminated %d", row.Process.PID))
			}
		}
		v.application.SetRoot(v.root, true)
		v.refresh()
	})
	// The modal owns the keyboard too, and for the sharper version of the same reason: without
	// this, the `k` that opened it would be read again as another kill.
	v.prompting = true
	v.application.SetRoot(modal, true)
}

// adjustPriority moves the selected process one step along the priority ladder.
//
// One step, because that is what F7 and F8 mean in htop. The result goes to the status line either
// way: most processes on a Windows machine cannot be opened for this at all -- they belong to
// SYSTEM or to another user -- and a keypress that silently does nothing is the failure this whole
// applet is written against.
func (v *topView) adjustPriority(step int) {
	v.rememberSelection()
	row, found := v.selectedRow()
	if !found {
		return
	}
	name, err := proc.AdjustPriority(row.Process.PID, step)
	if err != nil {
		v.status.SetText(fmt.Sprintf("[red]%v", err))
		return
	}
	v.status.SetText(fmt.Sprintf("[green]%s (pid %d) is now %s",
		row.Process.Name, row.Process.PID, name))
	v.refresh()
}

// promptFilter takes a filter interactively.
func (v *topView) promptFilter() {
	before := v.session.model.Filter
	field := tview.NewInputField().SetLabel("filter: ").SetText(before)
	field.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			v.session.model.Filter = field.GetText()
		} else {
			v.session.model.Filter = before
		}
		v.hidePrompt(field)
		v.refresh()
	})
	v.showPrompt(field)
}

// promptSearch searches as it is typed, leaving the list whole.
//
// Incremental, because that is what makes a search worth having over a filter: the table stays
// where it is and the cursor walks to the match, so the process is found *in context* -- with its
// neighbours, its parent, and its share of the machine still on screen.
func (v *topView) promptSearch() {
	before := v.session.model.Selected
	field := tview.NewInputField().SetLabel("search: ").SetText(v.session.model.Search)
	field.SetChangedFunc(func(text string) {
		v.session.model.Search = text
		// From the top on every keystroke, so the answer depends on what was typed and
		// not on how it was typed. Searching onwards from the cursor as the word grew
		// would walk forward through the table on every letter.
		v.moveToMatch(findTopMatch(v.rows, text, -1))
	})
	field.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			// Escape puts the cursor back, which is what makes trying a search free.
			v.session.model.Selected = before
			v.fillTable()
		}
		v.hidePrompt(field)
		v.refresh()
	})
	v.showPrompt(field)
}

// searchNext walks to the match after the cursor.
func (v *topView) searchNext() {
	term := v.session.model.Search
	if term == "" {
		v.promptSearch()
		return
	}
	row, _ := v.table.GetSelection()
	if !v.moveToMatch(findTopMatch(v.rows, term, row-1)) {
		v.status.SetText(fmt.Sprintf("[yellow]no process matches %q", term))
	}
}

// moveToMatch puts the cursor on a found row.
func (v *topView) moveToMatch(index int, found bool) bool {
	if !found {
		return false
	}
	v.session.model.Selected = v.rows[index].id()
	v.table.Select(index+1, 0)
	return true
}

// showPrompt puts an input field where the status line is.
//
// In the footer rather than over the whole screen, which is how this started: SetRoot on the field
// replaced the table with a single line, so a filter or a search was typed blind. The point of a
// search is to see where the cursor went.
func (v *topView) showPrompt(field *tview.InputField) {
	v.prompting = true
	v.root.RemoveItem(v.status)
	v.root.AddItem(field, 1, 0, true)
	v.application.SetFocus(field)
}

// hidePrompt puts the status line back.
func (v *topView) hidePrompt(field *tview.InputField) {
	v.prompting = false
	v.root.RemoveItem(field)
	v.root.AddItem(v.status, 1, 0, false)
	v.application.SetFocus(v.table)
}

// selectedRow is the row under the cursor.
func (v *topView) selectedRow() (topRow, bool) {
	row, _ := v.table.GetSelection()
	if row < 1 || row-1 >= len(v.rows) {
		return topRow{}, false
	}
	return v.rows[row-1], true
}

// topHelpText is the key list, shown in the status line rather than in a page of its own: a
// monitor's help is six words long and a full-screen help panel hides the thing being explained.
const topHelpText = "[white]q quit  / search  n next  F7/F8 priority  F4 filter  F5/t tree  H threads  K kernel  " +
	"I reverse  P/M/T/N sort  space tag  +/- fold  Z pause  p path  F9/k kill  r refresh"
