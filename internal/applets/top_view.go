package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
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
	summary := tview.NewTextView().SetDynamicColors(true)
	status := tview.NewTextView().SetDynamicColors(true)
	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(summary, topSummaryLines, 0, false).
		AddItem(table, 0, 1, true).
		AddItem(status, 1, 0, false)
	application.SetRoot(layout, true)

	view := &topView{
		session:     session,
		application: application,
		table:       table,
		summary:     summary,
		status:      status,
		stderr:      stderr,
		root:        layout,
	}
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

// topSummaryLines is how many rows the header occupies: the four lines writeTopSummary prints,
// plus one per processor bar, capped so a 64-core machine does not fill the screen with meters.
const topSummaryLines = 6

// topView is the drawn state, and the only place tview types appear.
type topView struct {
	session     *topSession
	application *tview.Application
	table       *tview.Table
	summary     *tview.TextView
	status      *tview.TextView
	stderr      io.Writer
	// root is the main layout, kept so a modal can be swapped in and back out again.
	root tview.Primitive
	rows []topRow
}

// refresh takes a sample and redraws.
func (v *topView) refresh() {
	snapshot, rates, rows, err := v.session.sample()
	if err != nil {
		v.status.SetText(fmt.Sprintf("[red]sampling failed: %v", err))
		return
	}
	v.rows = rows
	v.summary.SetText(topSummaryText(snapshot, rates))
	v.fillTable()
	v.status.SetText(topStatusText(v.session.model, len(rows)))
}

// fillTable writes the rows into the widget, keeping the selection on the same process.
//
// By pid rather than by row number, because the list reorders every second: a cursor that stayed
// on row nine would wander through the process list on its own, and F9 would kill whatever had
// arrived under it.
func (v *topView) fillTable() {
	selected := v.session.model.Selected
	v.table.Clear()
	for index, column := range v.session.model.Columns {
		cell := tview.NewTableCell(column.Header).
			SetStyle(tcell.StyleDefault.Bold(true).Reverse(true)).
			SetSelectable(false)
		if column.Right {
			cell.SetAlign(tview.AlignRight)
		}
		v.table.SetCell(0, index, cell)
	}
	selectedRow := 0
	for rowIndex, row := range v.rows {
		for columnIndex, column := range v.session.model.Columns {
			cell := tview.NewTableCell(column.Cell(row))
			if column.Right {
				cell.SetAlign(tview.AlignRight)
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

// topSummaryText is the header, with a bar per processor.
func topSummaryText(snapshot proc.Snapshot, rates proc.Rates) string {
	var out strings.Builder
	writeTopSummary(&out, snapshot, rates)
	for index, cpu := range rates.CPUs {
		if index >= topSummaryLines-4 {
			// Out of room. A machine with more processors than the header has lines
			// gets the total and the first few, which beats a header that pushes the
			// table off the screen.
			break
		}
		fmt.Fprintf(&out, "CPU%-3d %s\n", index, topBar(cpu.Busy, 40))
	}
	return out.String()
}

// topBar is a meter, drawn the way htop draws one.
func topBar(fraction float64, width int) string {
	filled := int(fraction * float64(width))
	if filled > width {
		filled = width
	}
	colour := "green"
	switch {
	case fraction > 0.9:
		colour = "red"
	case fraction > 0.6:
		colour = "yellow"
	}
	return fmt.Sprintf("[%s]%s[white]%s %s%%", colour, strings.Repeat("|", filled),
		strings.Repeat(" ", width-filled), strings.TrimSpace(topPercent(fraction)))
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
	return fmt.Sprintf("[white]%d rows  sort=%s%s  %s%s   [yellow]q[white] quit  "+
		"[yellow]F3[white] filter  [yellow]F5[white] tree  [yellow]F9[white] kill  [yellow]1-9[white] sort",
		rows, model.Sort, sortArrow(model.Descending), arrangement, filter)
}

func sortArrow(descending bool) string {
	if descending {
		return "-"
	}
	return "+"
}
