package applets

import (
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xiongnemo/nemosh/internal/textgrid"
)

// wget's progress meter is busybox's (libbb/progress.c, bb_progress_update), drawn on a console
// while a download is saved unless -q was given. It is redrawn at most once a second, and always
// once the size it expected has arrived. It shows:
//   - the file's name in twenty columns;
//   - how far through it is, and a bar as wide as the console leaves after 48 columns;
//   - the size so far in five characters;
//   - the time left, `--:--:--` when the size is not known, or `- stalled -` after five
//     seconds with nothing.
//
// Then a newline once it is done. busybox draws it on a pipe too, a line a second; this one draws
// it only on a console, where somebody is watching it.

const wgetStallSeconds = 5

type wgetProgress struct {
	out   io.Writer
	name  string
	width int
	clock func() time.Time
	base  time.Time
	// start, lastUpdate and lastChange are whole seconds since base, as busybox keeps them.
	start, lastUpdate, lastChange int64
	lastSize, transferred, total  int64
}

// newWgetProgress starts a meter for name, total bytes long or 0 when that is not known, on a
// console width columns wide.
func newWgetProgress(out io.Writer, name string, total int64, width int, clock func() time.Time) *wgetProgress {
	return &wgetProgress{out: out, name: wgetFixedWidth(name, 20), width: width, clock: clock, base: clock(), total: total}
}

// Write counts what is saved, and redraws when it is time to.
func (p *wgetProgress) Write(data []byte) (int, error) {
	p.transferred += int64(len(data))
	p.update()
	return len(data), nil
}

// finish draws the last state and ends the line, as busybox's PROGRESS_END does.
func (p *wgetProgress) finish() {
	if p.update() {
		fmt.Fprint(p.out, "\n")
	}
}

// update is bb_progress_update: it answers whether it drew.
func (p *wgetProgress) update() bool {
	elapsed := int64(p.clock().Sub(p.base) / time.Second)
	sinceUpdate := elapsed - p.lastUpdate
	p.lastUpdate = elapsed
	transferred := p.transferred
	if p.total != 0 && transferred >= p.total {
		transferred = p.total
	} else if sinceUpdate == 0 {
		return false
	}
	var line strings.Builder
	line.WriteString("\r" + p.name + " ")
	if p.total != 0 {
		fmt.Fprintf(&line, "%3d%% ", 100*transferred/p.total)
		if bar := min(p.width-48, 999); bar > 2 {
			stars := int(int64(bar) * transferred / p.total)
			line.WriteString("|" + strings.Repeat("*", stars) + strings.Repeat(" ", bar-stars) + "| ")
		}
	}
	line.WriteString(wgetSize5(uint64(transferred)))
	sinceChange := elapsed - p.lastChange
	if transferred != p.lastSize {
		p.lastChange, p.lastSize = elapsed, transferred
		if sinceChange >= wgetStallSeconds {
			// The stall is cut out of the time taken, as busybox cuts it.
			p.start += sinceChange
		}
		sinceChange = 0
	}
	taken := elapsed - p.start
	switch {
	case sinceChange >= wgetStallSeconds:
		line.WriteString("  - stalled -")
	case p.total == 0 || transferred == 0 || taken < 0:
		line.WriteString(" --:--:-- ETA")
	default:
		eta := min(p.total*taken/transferred-taken, 1000*60*60-1)
		fmt.Fprintf(&line, "%3d:%02d:%02d ETA", eta/3600, eta%3600/60, eta%60)
	}
	_, _ = io.WriteString(p.out, line.String())
	return true
}

// withMeter is where a body goes: dest, with the meter beside it when stderr is a console and
// -q was not given. finish ends the meter's line.
func (r wgetRequest) withMeter(dest, stderr io.Writer, name string, fetched *wgetFetched) (io.Writer, func()) {
	if r.quiet || !stdoutIsTerminal(stderr) {
		return dest, func() {}
	}
	// busybox's curfile is the last path element of the file it saves to.
	meter := newWgetProgress(stderr, path.Base(filepath.ToSlash(name)), fetched.knownLength(), TerminalColumns(stderr), time.Now)
	return io.MultiWriter(dest, meter), meter.finish
}

// knownLength is the body's size when the response gave one, and 0 when it is chunked or sent
// to the close, as busybox's meter has it.
func (f *wgetFetched) knownLength() int64 {
	if strings.EqualFold(f.headers["transfer-encoding"], "chunked") {
		return 0
	}
	length, err := strconv.ParseInt(f.headers["content-length"], 10, 64)
	if err != nil || length < 0 {
		return 0
	}
	return length
}

// wgetSize5 is busybox's smart_ulltoa5 with its " kMGTPEZY" scale: five characters, the number
// itself up to 99999, and above that tenths of a power of 1024 under 100, whole ones over.
func wgetSize5(value uint64) string {
	scale := 0
	if value > 99999 {
		value *= 10
		for {
			value /= 1024
			scale++
			if value < 100000 {
				break
			}
		}
	} else {
		return fmt.Sprintf("%5d", value)
	}
	whole, tenths := value/10, value%10
	unit := " kMGTPEZY"[scale]
	if whole >= 100 {
		return fmt.Sprintf("%4d%c", whole, unit)
	}
	return fmt.Sprintf("%2d.%d%c", whole, tenths, unit)
}

// wgetFixedWidth is name cut or padded to columns cells, as busybox's
// unicode_conv_to_printable_fixedwidth makes it.
func wgetFixedWidth(name string, columns int) string {
	var out strings.Builder
	used := 0
	for _, r := range name {
		cells := textgrid.RuneCells(r)
		if used+cells > columns {
			break
		}
		out.WriteRune(r)
		used += cells
	}
	return out.String() + strings.Repeat(" ", columns-used)
}
