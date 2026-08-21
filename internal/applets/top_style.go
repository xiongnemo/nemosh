package applets

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// Colour, and the meters that carry most of it.
//
// htop is readable at a glance because of colour rather than because of layout: the eye finds a red
// CPU figure without reading a single row, and a monochrome table of four hundred processes has to
// be read. So the colours here follow htop's scheme where it has one -- memory figures in cyan,
// idle rows dimmed, a running state in green -- and where it does not, the rule is that a number
// worth noticing is coloured by how big it is.
//
// Kept apart from top_view.go because that file is at its size ceiling, and because a colour scheme
// is the thing most likely to become configurable later.

// topLoadColour grades a fraction from idle to alarming.
//
// Grey at zero is htop's, and is the one that does most of the work: on a normal machine almost
// every process is using no CPU at all, so dimming them makes the handful that are stand out
// without any of the others having to be hidden.
func topLoadColour(fraction float64) tcell.Color {
	switch {
	case fraction <= 0:
		return tcell.ColorGray
	case fraction < 0.10:
		return tcell.ColorGreen
	case fraction < 0.35:
		return tcell.ColorYellow
	}
	return tcell.ColorRed
}

// topLoadTag is the same grading as a tview colour tag.
//
// Spelled out rather than taken from tcell.Color.String(), because the tag syntax is tview's and
// the name format is tcell's, and nothing promises the two agree.
func topLoadTag(fraction float64) string {
	switch {
	case fraction <= 0:
		return "gray"
	case fraction < 0.10:
		return "green"
	case fraction < 0.35:
		return "yellow"
	}
	return "red"
}

// topCellStyle is how one cell is drawn.
//
// By column key rather than by index, so a reordered or reconfigured layout keeps its colours.
func topCellStyle(key string, row topRow) tcell.Style {
	style := tcell.StyleDefault
	if row.Tagged {
		// Tagged rows are what a multiple kill acts on, so they are marked in the one way
		// that cannot be missed. htop uses the same reversed yellow.
		return style.Foreground(tcell.ColorBlack).Background(tcell.ColorYellow)
	}
	switch key {
	case "cpu":
		return style.Foreground(topLoadColour(row.Rate.CPU))
	case "mem":
		return style.Foreground(topLoadColour(row.MemoryShare))
	case "rss", "private", "commit":
		// Cyan for memory figures, as htop draws them: they are numbers you scan down a
		// column rather than read one of.
		return style.Foreground(tcell.ColorTeal)
	case "read", "write":
		if row.Rate.ReadBytesPerSecond+row.Rate.WriteBytesPerSecond == 0 {
			return style.Foreground(tcell.ColorGray)
		}
		return style.Foreground(tcell.ColorAqua)
	case "time":
		return style.Foreground(tcell.ColorSilver)
	case "user":
		return style.Foreground(tcell.ColorOlive)
	case "state":
		switch row.Process.State {
		case proc.StateRunning:
			return style.Foreground(tcell.ColorGreen).Bold(true)
		case proc.StateWaiting:
			// D is worth red for the reason htop makes it red: a process stuck in an
			// uninterruptible wait is the one you are looking for when the machine
			// feels stalled.
			return style.Foreground(tcell.ColorRed).Bold(true)
		}
	case "command":
		if row.Process.PID == 0 || row.Process.PID == 4 {
			// The two kernel processes. Present, because Idle holds the machine's spare
			// capacity and System its driver threads, but dimmed: neither is ever the
			// process someone opened a monitor to find.
			return style.Foreground(tcell.ColorGray)
		}
		return style.Bold(true)
	}
	return style
}

// topHeaderStyle draws the column headers, marking the one being sorted on.
//
// The mark matters more than it sounds: without it the only way to tell what the table is ordered
// by is to read the status line, and the sort column is the thing a person changes most often.
func topHeaderStyle(sorted bool) tcell.Style {
	style := tcell.StyleDefault.Foreground(tcell.ColorBlack).Bold(true)
	if sorted {
		return style.Background(tcell.ColorGreen)
	}
	return style.Background(tcell.ColorAqua)
}

// topStyleCell applies a style to a cell, painting the background only when the style names one.
//
// The transparency is not optional decoration. tview's NewTableCell sets Transparent, and a
// transparent cell's background is never painted -- so SetStyle with a background colour is
// accepted and silently ignored. That is why the coloured header first drew as plain text on the
// table's own black. Cells that name no background keep the table's, which is what they want.
func topStyleCell(cell *tview.TableCell, style tcell.Style) *tview.TableCell {
	cell.SetStyle(style)
	if _, background, _ := style.Decompose(); background != tcell.ColorDefault {
		cell.SetTransparency(false)
	}
	return cell
}

// topBars is a bar: the used part, then the unused part in a dimmer colour.
//
// Dots for the remainder rather than brackets around the whole. A literal `[` in a tview string
// starts a colour tag, and escaping it correctly in every position is a smaller prize than not
// having to think about it. And `|` and `.` rather than block characters for the reason this
// codebase keeps rediscovering: U+2588 and friends are East Asian Ambiguous, so their width depends
// on the font and the terminal, and alignment here has been broken by exactly that before.
func topBars(fraction float64, width int) string {
	if width < 4 {
		width = 4
	}
	filled := int(fraction*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return fmt.Sprintf("[%s]%s[gray]%s", topLoadTag(fraction),
		strings.Repeat("|", filled), strings.Repeat(".", width-filled))
}

// topUnknownBars is the bar for a rate that does not exist yet.
//
// An empty bar rather than no bar, and a dash rather than a zero. The first sample has nothing to
// take a rate against, so every CPU figure is *unknown* -- and 0.0% would be a lie that looks
// exactly like an idle machine, which is the worst available answer. Drawing the bars empty also
// keeps the header the height it is about to be, so the table does not jump down a line a second
// after it appears.
func topUnknownBars(width int) string {
	if width < 4 {
		width = 4
	}
	return "[gray]" + strings.Repeat(".", width)
}

// topMeter is one of the three meters for the machine as a whole, with its figures beside it.
func topMeter(label string, fraction float64, width int, note string) string {
	return topMeterText(label, topBars(fraction, width-topMeterOverhead),
		strings.TrimSpace(topPercent(fraction))+"%", note)
}

// topUnknownMeter is the same meter before there is a rate to put in it.
func topUnknownMeter(label string, width int, note string) string {
	return topMeterText(label, topUnknownBars(width-topMeterOverhead), "--", note)
}

// topMeterText lays out a wide meter. One place, so the known and unknown forms cannot drift into
// different widths and make the header ripple when the first rate arrives.
func topMeterText(label, bars, percent, note string) string {
	out := fmt.Sprintf("[white]%-4s%s [white]%6s", label, bars, percent)
	if note != "" {
		out += "  [silver]" + note
	}
	return out
}

// topSmallMeter is the compact per-processor form.
//
// Compact so that a sixteen-core machine's meters fit in the lines the header spends on them. The
// wide form fits two to a line, which would show four cores out of sixteen.
func topSmallMeter(label string, fraction float64, width int) string {
	return fmt.Sprintf("[white]%-3s%s [white]%6s", label, topBars(fraction, width),
		strings.TrimSpace(topPercent(fraction))+"%")
}

// topSmallUnknownMeter is the compact form before its rate exists. Same width as topSmallMeter,
// so nothing moves sideways when the figures arrive.
func topSmallUnknownMeter(label string, width int) string {
	return fmt.Sprintf("[white]%-3s%s [gray]%6s", label, topUnknownBars(width), "--")
}

// topProcessorLayout is how the per-processor meters are arranged.
//
// Every processor gets one, which is htop's behaviour and the right one: a monitor that shows half
// the cores is answering a different question from the one asked. So the header's height follows
// the machine rather than being a constant -- but not past a third of the screen, because on a
// sixty-four-core box the meters would otherwise leave no room for the table they annotate.
func topProcessorLayout(count, width, height int) (perLine, lines int) {
	perLine = max(width/topMeterCells, 1)
	lines = (count + perLine - 1) / perLine
	if limit := max((height-topSummaryFixedLines)/3, 1); lines > limit {
		lines = limit
	}
	return perLine, lines
}

// topSummaryHeight is how many rows the header needs, which the layout is resized to.
func topSummaryHeight(count, width, height int) int {
	_, lines := topProcessorLayout(count, width, height)
	return topSummaryFixedLines + lines
}

// topSummaryText is the header: the machine as a whole, in meters.
//
// width and height are the terminal's, so the meters fill the window and follow a resize rather
// than sitting at a size guessed when the program started.
func topSummaryText(snapshot proc.Snapshot, rates proc.Rates, width, height int) string {
	if width <= 0 {
		width = lsDefaultWidth
	}
	running, threads := 0, 0
	for _, process := range snapshot.Processes {
		threads += process.Threads
		if process.State == proc.StateRunning {
			running++
		}
	}
	memory := snapshot.Memory
	var out strings.Builder
	fmt.Fprintf(&out, "[white]top - up [aqua]%s[white], [aqua]%d[white] processes, [aqua]%d[white] threads, [aqua]%d[white] running\n",
		topUptime(snapshot.Uptime), len(snapshot.Processes), threads, running)
	// Interval zero means there was no usable pair of samples to take a rate from, which is
	// always true of the first one. Said out loud rather than drawn as zero: an idle-looking
	// machine and an unmeasured one are different claims.
	note := fmt.Sprintf("%d processors", len(snapshot.CPUs))
	if rates.Interval <= 0 {
		fmt.Fprintln(&out, topUnknownMeter("CPU", width, note+"  [yellow]sampling..."))
	} else {
		fmt.Fprintln(&out, topMeter("CPU", rates.TotalBusy, width, note))
	}
	fmt.Fprintln(&out, topMeter("Mem", memory.Share(), width, fmt.Sprintf("%s of %s, %s cached",
		topBytes(memory.UsedPhysical()), topBytes(memory.TotalPhysical), topBytes(memory.Cached))))
	// Commit rather than swap, as the plain form also says: Windows promises memory rather than
	// paging it out in a way that can be counted the way Linux counts swap.
	fmt.Fprintln(&out, topMeter("Cmt", memory.CommitShare(), width, fmt.Sprintf("%s of %s",
		topBytes(memory.CommitTotal), topBytes(memory.CommitLimit))))
	writeTopProcessorMeters(&out, snapshot, rates, width, height)
	return out.String()
}

// writeTopProcessorMeters packs one meter per processor into the lines it was given.
//
// The count comes from the snapshot rather than from the rates, because the snapshot knows how many
// processors there are on its first call while a rate needs two. So the meters are all present from
// the start, empty until there is something to put in them.
func writeTopProcessorMeters(out *strings.Builder, snapshot proc.Snapshot, rates proc.Rates, width, height int) {
	count := len(snapshot.CPUs)
	if count == 0 {
		return
	}
	bar := topMeterCells - topSmallOverhead
	perLine, lines := topProcessorLayout(count, width, height)
	for index := range count {
		if index >= perLine*lines {
			break
		}
		if index < len(rates.CPUs) {
			fmt.Fprint(out, topSmallMeter(strconv.Itoa(index), rates.CPUs[index].Busy, bar))
		} else {
			fmt.Fprint(out, topSmallUnknownMeter(strconv.Itoa(index), bar))
		}
		if index%perLine == perLine-1 || index == count-1 {
			fmt.Fprintln(out)
			continue
		}
		fmt.Fprint(out, "  ")
	}
}

const (
	// topMeterOverhead is the label, the percentage and the note beside a wide meter, so the bar
	// takes what is left of the terminal rather than a guess at it. A bar sized without this
	// wraps onto the next line, which in a fixed-height header pushes a processor meter off.
	topMeterOverhead = 40
	// topSmallOverhead is the same for the compact form: three for the label, seven for the
	// percentage, two for the gap to the next meter.
	topSmallOverhead = 12
	// topMeterCells is how much room one per-processor meter takes.
	topMeterCells = 20
	// topSummaryFixedLines is the uptime line and the three machine meters. What the header
	// spends on per-processor meters is on top of this and depends on the machine.
	topSummaryFixedLines = 4
	// topSelectionAbsent is "no process selected". Not zero: pid 0 is Idle, a row the cursor
	// can sit on, so zero cannot mean absent. A cursor that jumped to the top row every second
	// was this exact confusion.
	topSelectionAbsent = -1
)
