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
	case topActionNice:
		v.status.SetText("[yellow]priority: not yet wired; see docs/design/process-view.md")
	case topActionHelp:
		v.status.SetText(topHelpText)
	case topActionFilterPrompt:
		v.promptFilter()
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
	v.application.SetRoot(modal, true)
}

// promptFilter takes a filter interactively.
func (v *topView) promptFilter() {
	field := tview.NewInputField().SetLabel("filter: ").SetText(v.session.model.Filter)
	field.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			v.session.model.Filter = field.GetText()
		}
		v.application.SetRoot(v.root, true)
		v.refresh()
	})
	v.application.SetRoot(field, true)
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
const topHelpText = "[white]q quit  F3// filter  F5/t tree  H threads  K kernel procs  " +
	"I reverse  space fold  1-9 sort by column  F9/k kill  r refresh now"
