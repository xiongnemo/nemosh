package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// The drawn form.
//
// The hard part was not the widgets, it was getting a terminal at all. An applet is handed an
// io.Reader and an io.Writer by the fd table, so that redirection works, and tcell needs real
// console handles. Both seams for that already existed and were built for other reasons:
// `descriptorWriter.TerminalFile` was added so `ls` could decide whether to lay out columns, and
// the stdin lease was added so an external process could be handed the real console. A monitor
// needs exactly those two, which is the argument for having put them in the fd table rather than
// in the callers.
//
// Without a terminal this is not reached at all: the applet prints one plain sample instead. So
// there is no "refuses to run in a pipe" case to handle -- there is a different output shape.

// stdinFile is what a stream implements when it can lease out the real console for the duration
// of a call. The runtime's descriptorReader does; see internal/shell/runtime/external_stdin.go,
// where an external process gets the same treatment for the same reason.
type stdinFile interface {
	LeaseStdinFile(context.Context) (*os.File, func(), bool)
}

// runTopInteractive draws until the user quits.
func runTopInteractive(ctx context.Context, options topOptions, stdin io.Reader, stdout, stderr io.Writer) error {
	input, release, ok := leaseTopStdin(ctx, stdin)
	if !ok {
		// stdout is a terminal but stdin is not -- `top < /dev/null` on a terminal. There
		// is no way to take a key press, so the plain form is the honest answer, and the
		// reason is worth a line: this is the case that looks most like a bug.
		fmt.Fprintln(stderr, "top: standard input is not a terminal, so no key can be read; printing one sample")
		return runTopBatch(ctx, options, stdout)
	}
	// The lease is taken for its side effect as much as for the file: the shell's own reader
	// thread may be parked on a console read, and two readers on one console input handle means
	// keys go to whichever asks first. Leasing stops that thread for the duration.
	_ = input
	defer release()
	// NewScreen rather than a screen built over the leased file. On Windows tcell drives the
	// console API through the process's own handles, and those are this terminal -- the fd
	// table only sends fd 1 elsewhere when it was redirected, and a redirected top took the
	// plain path above and never got here.
	screen, err := tcell.NewScreen()
	if err != nil {
		// tcell could not drive this terminal. Say so and fall back rather than failing:
		// the numbers are worth having even without the drawing.
		fmt.Fprintf(stderr, "top: cannot drive this terminal (%v); printing one sample\n", err)
		return runTopBatch(ctx, options, stdout)
	}
	return runTopApplication(ctx, options, screen, stderr, nil)
}

// leaseTopStdin borrows the real console for as long as the monitor runs.
func leaseTopStdin(ctx context.Context, stdin io.Reader) (*os.File, func(), bool) {
	if file, ok := stdin.(*os.File); ok {
		return file, func() {}, true
	}
	leaser, ok := stdin.(stdinFile)
	if !ok {
		return nil, func() {}, false
	}
	return leaser.LeaseStdinFile(ctx)
}

// runTopApplication is the event loop: a ticker samples, key presses change the view.
//
// ready, when given, receives the application once it is built. Only the headless test uses it,
// and it uses it for a reason that is not a convenience: the screen may only be read from tview's
// own goroutine, so a test needs the application in order to queue a read onto it.
func runTopApplication(ctx context.Context, options topOptions, screen tcell.Screen, stderr io.Writer,
	ready chan<- *tview.Application) error {
	session := newTopSession(options)
	application := tview.NewApplication().SetScreen(screen)
	table := tview.NewTable().SetFixed(1, 0).SetSelectable(true, false)
	table.SetSelectedStyle(tcell.StyleDefault.Reverse(true))
	// No wrapping: this is a fixed-height header, so an overlong line must be cut off rather
	// than reflowed into the space a processor meter was going to use.
	summary := tview.NewTextView().SetDynamicColors(true).SetWrap(false)
	status := tview.NewTextView().SetDynamicColors(true)
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(summary, topSummaryFixedLines, 0, false).
		AddItem(table, 0, 1, true).
		AddItem(status, 1, 0, false)
	application.SetRoot(layout, true)

	width, height := screen.Size()
	view := &topView{
		session:       session,
		application:   application,
		table:         table,
		summary:       summary,
		status:        status,
		stderr:        stderr,
		root:          layout,
		summaryWidth:  width,
		summaryHeight: height,
	}
	// Where the meters learn the terminal's size. Asking the widget was the obvious thing and it
	// was wrong: a tview Box reports a default 15x10 rect until it has been laid out, so a
	// GetInnerRect before the first draw answers 15 rather than answering zero -- a plausible
	// number, silently, which drew every meter four cells wide. The screen knows its own size
	// before anything is drawn, and the layout's draw function keeps up with a resize.
	layout.SetDrawFunc(func(_ tcell.Screen, x, y, width, height int) (int, int, int, int) {
		if width != view.summaryWidth || height != view.summaryHeight {
			view.summaryWidth, view.summaryHeight = width, height
			view.drawSummary()
		}
		return x, y, width, height
	})
	// The cursor's own movement keys are tview's -- arrows, page up and down, home and end are
	// handled by the table and never reach the input capture. So this is the only place the model
	// can learn where the cursor went, and without it the selection was remembered only when some
	// *other* key was pressed. See rememberSelection for what that cost.
	table.SetSelectionChangedFunc(view.selectionChanged)
	view.refresh()
	application.SetInputCapture(view.key)
	if ready != nil {
		ready <- application
	}

	// The ticker is stopped by the application returning, and the goroutine ends with the
	// context: a monitor that leaves a timer running after `q` would keep the shell awake.
	ticker := time.NewTicker(options.delay)
	defer ticker.Stop()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				application.Stop()
				return
			case <-ticker.C:
				// QueueUpdateDraw, because a sample taken on this goroutine and
				// written into widgets would race the draw happening on tview's.
				application.QueueUpdateDraw(view.refresh)
			}
		}
	}()
	err := application.Run()
	close(done)
	return err
}

// topView is the drawn state, and the only place tview types appear.
type topView struct {
	session     *topSession
	application *tview.Application
	table       *tview.Table
	summary     *tview.TextView
	status      *tview.TextView
	stderr      io.Writer
	// root is the main layout, kept so a modal can be swapped in and back out again and so the
	// header can be resized to the machine.
	root *tview.Flex
	// filling is set while the table is being rebuilt, so the selection callback ignores the
	// row numbers it sees in the middle of that.
	filling bool
	// summaryWidth and summaryHeight are the terminal's, so the meters fill the window and
	// follow a resize.
	summaryWidth  int
	summaryHeight int
	// snapshot and rates are the last sample, kept so the header can be redrawn at a new width
	// without taking another one.
	snapshot proc.Snapshot
	rates    proc.Rates
	rows     []topRow
}

// refresh takes a sample and redraws.
//
// Paused means paused: the sample is not taken at all, rather than taken and hidden. Taking it
// would keep the rates moving underneath, so unpausing would jump.
func (v *topView) refresh() {
	if v.session.model.Paused {
		v.status.SetText(topStatusText(v.session.model, len(v.rows)))
		return
	}
	snapshot, rates, rows, err := v.session.sample()
	if err != nil {
		v.status.SetText(fmt.Sprintf("[red]sampling failed: %v", err))
		return
	}
	v.rows = rows
	v.snapshot, v.rates = snapshot, rates
	v.drawSummary()
	v.fillTable()
	v.status.SetText(topStatusText(v.session.model, len(rows)))
}

// drawSummary writes the header at the current width.
func (v *topView) drawSummary() {
	v.summary.SetText(topSummaryText(v.snapshot, v.rates, v.summaryWidth, v.summaryHeight))
	// The header is as tall as the machine needs, so the layout is resized rather than the
	// meters being cropped to a constant. htop grows its header for the same reason.
	v.root.ResizeItem(v.summary,
		topSummaryHeight(len(v.rates.CPUs), v.summaryWidth, v.summaryHeight), 0)
}

// fillTable writes the rows into the widget, keeping the selection on the same process.
//
// By pid rather than by row number, because the list reorders every second: a cursor that stayed
// on row nine would wander through the process list on its own, and F9 would kill whatever had
// arrived under it.
func (v *topView) fillTable() {
	selected := v.session.model.Selected
	// The callback fires while the table is being rebuilt, with row numbers that mean nothing
	// yet, and letting it record one would overwrite the selection this is trying to restore.
	v.filling = true
	defer func() { v.filling = false }()
	v.table.Clear()
	for index, column := range v.session.model.Columns {
		cell := topStyleCell(tview.NewTableCell(column.Header),
			topHeaderStyle(column.Key == v.session.model.Sort)).SetSelectable(false)
		if column.Right {
			cell.SetAlign(tview.AlignRight)
		}
		// Width zero is the column table's way of saying "the rest of the line", which is
		// the command and nothing else. Expanding it is what makes the table fill the
		// window: tview sizes a column to its widest cell, so without this a table of short
		// names sat in the left third of a maximised terminal.
		if column.Width == 0 {
			cell.SetExpansion(1)
		}
		v.table.SetCell(0, index, cell)
	}
	selectedRow := 0
	for rowIndex, row := range v.rows {
		for columnIndex, column := range v.session.model.Columns {
			cell := topStyleCell(tview.NewTableCell(column.Cell(row)), topCellStyle(column.Key, row))
			if column.Right {
				cell.SetAlign(tview.AlignRight)
			}
			if column.Width == 0 {
				cell.SetExpansion(1)
			}
			v.table.SetCell(rowIndex+1, columnIndex, cell)
		}
		if row.Process.PID == selected {
			selectedRow = rowIndex + 1
		}
	}
	if selectedRow > 0 {
		v.table.Select(selectedRow, 0)
	}
}

// selectionChanged records which process the cursor moved onto.
//
// This is the fix for a cursor that jumped back to the top row every second. The selection was
// only recorded when a key the model handled was pressed, so plain arrow movement left it at the
// zero value -- and zero is a real pid here, Idle's, so every refresh dutifully found Idle and put
// the cursor back on it. Hence topSelectionAbsent rather than zero for "nothing selected": on a
// platform where pid 0 is a process you can see, zero cannot mean absent.
func (v *topView) selectionChanged(row, _ int) {
	if v.filling {
		return
	}
	if row < 1 || row-1 >= len(v.rows) {
		return
	}
	v.session.model.Selected = v.rows[row-1].Process.PID
}

// topStatusText is the key hint line, which is what makes a monitor discoverable at all.
func topStatusText(model topModel, rows int) string {
	arrangement := "flat"
	if model.Tree {
		arrangement = "tree"
	}
	filter := ""
	if model.Filter != "" {
		filter = fmt.Sprintf("  filter=%q", model.Filter)
	}
	paused := ""
	if model.Paused {
		paused = "  [red]PAUSED"
	}
	return fmt.Sprintf("[white]%d rows  sort=%s%s  %s%s%s   [yellow]q[white] quit  "+
		"[yellow]F4[white] filter  [yellow]F5[white] tree  [yellow]P/M/T[white] sort  "+
		"[yellow]F9[white] kill  [yellow]F1[white] help",
		rows, model.Sort, sortArrow(model.Descending), arrangement, filter, paused)
}

func sortArrow(descending bool) string {
	if descending {
		return "-"
	}
	return "+"
}
