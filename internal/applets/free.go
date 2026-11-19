package applets

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// free: what the machine has and what is left.
//
// Nearly all of this already existed. internal/proc.Memory is what `top` draws
// its meters from, so this applet is a formatter rather than a measurement --
// which also means the two cannot disagree about how much memory the machine has.
//
// The column layout is busybox's, measured 2026-08-22: a label of seven and values of twelve.
// The header was printed five narrower than this and the rows one, out of line with each other.
//
//	              total        used        free      shared  buff/cache   available
//	Mem:       66833188    32505304    16598704           0    17729180           0
//	Swap:      21573812     9244800    12329012
//
// procps-ng's -t adds a Total row, -w splits buffers from cache, and -s and -c print the table
// again every so many seconds or so many times, a blank line between, as Ubuntu's free does.

// freeScale is the unit an option selects, as busybox's free reads it: the first of -b -k -m -g
// -h given, each value rounded to the nearest, and -h each in the largest unit that leaves it
// under 1024, with one decimal, `63.7G`. Kilobytes are the default, which is what both references
// do and why `free` numbers look large. The values were cut short rather than rounded, -h was
// taken and ignored, and of several options the first of b, m and g in that order won.
type freeScale struct {
	unit uint64
}

// freeLayout is procps' rows and columns: total is -t, wide is -w.
type freeLayout struct {
	total, wide bool
}

const (
	freeFlags       = "bkmghtw"
	freeValued      = "sc"
	freeLongOptions = "bytes=b kibi=k mebi=m gibi=g human=h total=t wide=w seconds=s count=c"
)

func newFreeApplet() Applet {
	return simpleApplet{name: "free", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletLongOptions(ctx, args, freeLongOptions, freeFlags, freeValued)
		if err != nil {
			return err
		}
		if len(operands) > 0 {
			return fmt.Errorf("extra operand '%s'", operands[0])
		}
		delay, count, err := freeRepeat(options)
		if err != nil {
			return err
		}
		layout := freeLayout{total: options.has('t'), wide: options.has('w')}
		sampler := proc.NewSampler()
		for printed := 0; count == 0 || printed < count; printed++ {
			if printed > 0 {
				// procps sleeps between tables, and Ctrl-C ends the wait.
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
				if _, err := fmt.Fprintln(stdout); err != nil {
					return err
				}
			}
			// The same sampler `top` uses, so the two cannot disagree about how much
			// memory the machine has.
			snapshot, err := sampler.Sample(false)
			if err != nil {
				return err
			}
			if err := writeFreeTable(stdout, snapshot.Memory, freeScaleFor(options), layout); err != nil {
				return err
			}
		}
		return nil
	}}
}

// freeRepeat is -s's delay and -c's count, as procps reads them: -s alone repeats until it is
// stopped, -c alone a second apart, and neither prints the table once.
func freeRepeat(options appletOptions) (time.Duration, int, error) {
	delay, count := time.Second, 1
	if options.has('s') {
		seconds, err := strconv.ParseFloat(options.value('s'), 64)
		if err != nil || seconds <= 0 {
			return 0, 0, fmt.Errorf("seconds argument '%s' failed", options.value('s'))
		}
		delay, count = time.Duration(seconds*float64(time.Second)), 0
	}
	if options.has('c') {
		value, err := strconv.Atoi(options.value('c'))
		if err != nil || value < 1 {
			return 0, 0, fmt.Errorf("count argument '%s' failed", options.value('c'))
		}
		count = value
	}
	return delay, count, nil
}

func freeScaleFor(options appletOptions) freeScale {
	for _, letter := range options.order {
		switch letter {
		case 'b':
			return freeScale{unit: 1}
		case 'k':
			return freeScale{unit: 1024}
		case 'm':
			return freeScale{unit: 1 << 20}
		case 'g':
			return freeScale{unit: 1 << 30}
		case 'h':
			return freeScale{unit: 0}
		}
	}
	return freeScale{unit: 1024}
}

func (s freeScale) of(value uint64) string { return humanReadable(value, 1, s.unit) }

// writeFreeTable prints the rows busybox prints, and procps' Total under -t.
//
// The Swap row is Windows' *commit charge*, not a swap file. There is no swap
// partition to measure the way Linux measures one, and commit is the closest
// honest substitute -- it is the number that says whether the next allocation
// will be refused. internal/proc/sample.go:148 records the same reasoning for
// `top`, and the two now agree because they read the same field.
//
// `shared` is always zero: Windows has no equivalent counter, and inventing one
// from working-set overlap would be a guess presented as a measurement. Nor has it
// buffers apart from its cache, so -w's buffers are zero and its cache the standby
// and modified lists, as buff/cache is.
func writeFreeTable(stdout io.Writer, memory proc.Memory, scale freeScale, layout freeLayout) error {
	header := fmt.Sprintf("       %12s%12s%12s%12s%12s%12s", "total", "used", "free", "shared", "buff/cache", "available")
	mem := fmt.Sprintf("Mem:   %12s%12s%12s%12s%12s%12s",
		scale.of(memory.TotalPhysical), scale.of(memory.UsedPhysical()), scale.of(memory.AvailablePhysical),
		scale.of(0), scale.of(memory.Cached), scale.of(memory.AvailablePhysical))
	if layout.wide {
		header = fmt.Sprintf("       %12s%12s%12s%12s%12s%12s%12s", "total", "used", "free", "shared", "buffers", "cache", "available")
		mem = fmt.Sprintf("Mem:   %12s%12s%12s%12s%12s%12s%12s",
			scale.of(memory.TotalPhysical), scale.of(memory.UsedPhysical()), scale.of(memory.AvailablePhysical),
			scale.of(0), scale.of(0), scale.of(memory.Cached), scale.of(memory.AvailablePhysical))
	}
	// Commit beyond physical memory is what is backed by the page file, which is
	// the closest thing to "swap used" that Windows reports.
	swapUsed := uint64(0)
	if memory.CommitTotal > memory.UsedPhysical() {
		swapUsed = memory.CommitTotal - memory.UsedPhysical()
	}
	swapTotal := uint64(0)
	if memory.CommitLimit > memory.TotalPhysical {
		swapTotal = memory.CommitLimit - memory.TotalPhysical
	}
	swapFree := uint64(0)
	if swapTotal > swapUsed {
		swapFree = swapTotal - swapUsed
	}
	table := header + "\n" + mem + "\n" +
		fmt.Sprintf("Swap:  %12s%12s%12s\n", scale.of(swapTotal), scale.of(swapUsed), scale.of(swapFree))
	if layout.total {
		table += fmt.Sprintf("Total: %12s%12s%12s\n", scale.of(memory.TotalPhysical+swapTotal),
			scale.of(memory.UsedPhysical()+swapUsed), scale.of(memory.AvailablePhysical+swapFree))
	}
	_, err := io.WriteString(stdout, table)
	return err
}
