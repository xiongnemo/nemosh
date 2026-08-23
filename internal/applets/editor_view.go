package applets

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// The editor's chrome: a title line, the text area, a message line and the key
// legend.
//
// nano's layout, which is the one the name promises: title at the top, two lines
// of keys at the bottom, and a message line just above them where prompts and
// errors appear.

type editorView struct {
	session     *editorSession
	keys        editorKeyMap
	application *tview.Application
	layout      *tview.Flex
	// area is a TextArea that colours itself. The type embeds tview's, so every call
	// through it is unchanged; what it adds is a Draw that re-colours what tview drew.
	area    *highlightedArea
	title   *tview.TextView
	message *tview.TextView
	legend  *tview.TextView
	// cut holds the last line taken by the cut key, so paste can put it back.
	cut string
	// modified is tracked here rather than read from the area, because "has this
	// changed since it was saved" is not something the widget knows.
	modified bool
	// legendWidth is the width the key legend was last laid out for, so a redraw at
	// the same size does no work.
	legendWidth int
	// prompt, when set, is receiving a line of input rather than the buffer.
	prompt func(answer string)
	editorPromptState
}

func newEditorView(session *editorSession, keys editorKeyMap, application *tview.Application) *editorView {
	view := &editorView{session: session, keys: keys, application: application}
	// The language comes from the file name, and a name that matches nothing gets no
	// highlighting rather than a guess -- `notes.txt` should look like text.
	view.area = newHighlightedArea(highlightSyntaxFor(session.path))
	view.area.SetText(session.text, false)
	// Lexed once for the file as opened, and again on every change. Not on scroll:
	// scrolling cannot change what a line means, and re-lexing on it would make
	// paging through a large file cost a full pass per keystroke.
	view.area.relex()
	view.area.SetChangedFunc(func() {
		view.modified = true
		view.area.relex()
		view.refreshTitle()
	})
	view.title = tview.NewTextView().SetDynamicColors(true)
	view.message = tview.NewTextView().SetDynamicColors(true)
	view.legend = tview.NewTextView().SetDynamicColors(true)

	// A title bar rather than a line of text, which is what nano has and what makes the
	// top of the screen read as belonging to the editor. Without a background the row
	// was twenty characters on an otherwise empty line, and the window looked like it
	// was not using the terminal.
	view.title.SetBackgroundColor(tcell.ColorNavy)
	view.title.SetTextColor(tcell.ColorWhite)

	view.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(view.title, 1, 0, false).
		AddItem(view.area, 0, 1, true).
		AddItem(view.message, 1, 0, false).
		AddItem(view.legend, len(keys.footer(80)), 0, false)
	// The legend is laid out for the width it is actually given, and asked again
	// whenever that changes -- a resize, or a terminal that was never 80 columns to
	// begin with. It used to be laid out exactly once, for a hardcoded 80.
	view.legend.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		view.layoutFooter(width)
		return x, y, width, height
	})
	// Once up front as well, so the first frame is right rather than right on the second.
	view.layoutFooter(80)
	view.refreshTitle()
	view.area.SetInputCapture(view.handleKey)
	return view
}

// layoutFooter lays the key legend out for a width, and does nothing if the width has
// not changed.
//
// The guard is what makes it safe to call from a draw function: setting a TextView's
// text is a mutation, and doing it on every frame regardless would be work per redraw
// for a line that only changes when the window does.
func (v *editorView) layoutFooter(width int) {
	if width <= 0 || width == v.legendWidth {
		return
	}
	v.legendWidth = width
	lines := v.keys.footer(width)
	v.legend.SetText(strings.Join(lines, "\n"))
	// The row count depends on the width -- seven labels are one row on a wide terminal
	// and two on a narrow one -- so the height the layout gives the legend has to
	// follow. Without this it was allotted two rows forever, and a wide terminal drew
	// one row of keys above a blank one.
	//
	// Resizing from inside a draw function means this frame uses the old height and the
	// next uses the new: one frame of lag on a resize, which is invisible. tview draws
	// on a single goroutine, so there is no race to have.
	v.layout.ResizeItem(v.legend, len(lines), 0)
}

// refreshTitle redraws the top line: the editor's name, the file, and whether
// there are unsaved changes.
//
// "Modified" is shown because an editor that loses work silently is the worst
// kind, and it is the one piece of state a user cannot otherwise see.
func (v *editorView) refreshTitle() {
	// The base name, not the whole path: nano shows the file name, and a Windows
	// path is long enough to fill the title line on its own -- an 80-column
	// terminal showed `C:` and nothing else.
	name := filepathBase(v.session.path)
	switch {
	case name == "":
		name = "New Buffer"
	case v.session.isNew:
		name += " (new)"
	}
	flags := ""
	if v.modified {
		flags = "  [yellow]Modified[-]"
	}
	if v.session.readOnly {
		flags += "  [red]Read-Only[-]"
	}
	v.title.SetText(fmt.Sprintf("  nemosh %s   [::b]%s[::-]%s", v.keys.name, name, flags))
}

func (v *editorView) setMessage(format string, args ...any) {
	v.message.SetText(fmt.Sprintf(format, args...))
}

// handleKey is the input capture. It returns nil to swallow a key and the event
// to let the text area have it.
func (v *editorView) handleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.prompt != nil {
		return v.handlePromptKey(event)
	}
	switch v.keys.lookup(event) {
	case editorSave:
		v.save()
		return nil
	case editorQuit:
		v.quit()
		return nil
	case editorSearch:
		v.ask("Search: ", v.search)
		return nil
	case editorCutLine:
		v.cutLine()
		return nil
	case editorPasteLine:
		v.pasteLine()
		return nil
	case editorHelp:
		v.showHelp()
		return nil
	case editorGoToLine:
		v.ask("Go to line: ", v.goToLine)
		return nil
	}
	if v.session.readOnly && isEditingKey(event) {
		v.setMessage("[red]Read-only: the buffer cannot be changed[-]")
		return nil
	}
	v.setMessage("")
	return event
}

// isEditingKey reports whether a key would change the buffer, which is what
// read-only has to stop while still allowing movement.
func isEditingKey(event *tcell.EventKey) bool {
	switch event.Key() {
	case tcell.KeyRune, tcell.KeyEnter, tcell.KeyTab,
		tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete:
		return true
	}
	return false
}

func (v *editorView) save() {
	if err := v.session.save(v.area.GetText()); err != nil {
		v.setMessage("[red]%v[-]", err)
		return
	}
	v.modified = false
	v.refreshTitle()
	v.setMessage("Wrote %s", v.session.path)
}

// quit refuses to lose unsaved work on the first press and accepts on the second,
// which is the compromise nano makes: a prompt needs a yes/no reader, and losing
// a buffer to one keystroke is the outcome worth preventing.
func (v *editorView) quit() {
	if v.modified {
		v.modified = false
		v.refreshTitle()
		v.setMessage("[yellow]Unsaved changes. Press the exit key again to discard them.[-]")
		return
	}
	v.application.Stop()
}

// cutLine takes the line the cursor is on, in the manner of nano's ^K.
func (v *editorView) cutLine() {
	lines, row := v.lines()
	if row >= len(lines) {
		return
	}
	v.cut = lines[row]
	remaining := append(append([]string{}, lines[:row]...), lines[row+1:]...)
	v.setText(strings.Join(remaining, "\n"), row)
	v.setMessage("Cut one line")
}

func (v *editorView) pasteLine() {
	if v.cut == "" {
		v.setMessage("Nothing to paste")
		return
	}
	lines, row := v.lines()
	restored := append([]string{}, lines[:row]...)
	restored = append(restored, v.cut)
	restored = append(restored, lines[row:]...)
	v.setText(strings.Join(restored, "\n"), row+1)
	v.setMessage("Pasted one line")
}

// search moves the cursor to the next line containing the text, wrapping once.
//
// Line granularity rather than character: it is what makes the feature useful for
// finding a place in a file, and claiming more would need a match highlight this
// does not draw.
func (v *editorView) search(needle string) {
	if needle == "" {
		return
	}
	lines, row := v.lines()
	for offset := 1; offset <= len(lines); offset++ {
		candidate := (row + offset) % len(lines)
		if strings.Contains(lines[candidate], needle) {
			v.moveToLine(candidate)
			v.setMessage("Found on line %d", candidate+1)
			return
		}
	}
	v.setMessage("[yellow]%q not found[-]", needle)
}

func (v *editorView) goToLine(answer string) {
	number, err := strconv.Atoi(strings.TrimSpace(answer))
	if err != nil || number < 1 {
		v.setMessage("[red]not a line number: %s[-]", answer)
		return
	}
	lines, _ := v.lines()
	if number > len(lines) {
		number = len(lines)
	}
	v.moveToLine(number - 1)
	v.setMessage("Line %d", number)
}

func (v *editorView) showHelp() {
	var help strings.Builder
	help.WriteString("Keys: ")
	for index, binding := range v.keys.bindings {
		if index > 0 {
			help.WriteString("  ")
		}
		help.WriteString(binding.label)
	}
	v.setMessage("%s", help.String())
}

// ask collects a line of input on the message row, which is where nano puts its
// prompts.
func (v *editorView) ask(label string, answer func(string)) {
	v.prompt = answer
	v.promptLabel, v.promptText = label, ""
	v.setMessage("%s", label)
}

func (v *editorView) handlePromptKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		answer, handler := v.promptText, v.prompt
		v.prompt, v.promptText = nil, ""
		handler(answer)
	case tcell.KeyEscape:
		v.prompt, v.promptText = nil, ""
		v.setMessage("Cancelled")
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if v.promptText != "" {
			runes := []rune(v.promptText)
			v.promptText = string(runes[:len(runes)-1])
		}
		v.setMessage("%s%s", v.promptLabel, v.promptText)
	case tcell.KeyRune:
		v.promptText += string(event.Rune())
		v.setMessage("%s%s", v.promptLabel, v.promptText)
	}
	return nil
}
