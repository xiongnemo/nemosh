package applets

import (
	"context"
	"fmt"
	"io"

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

// freeScale is the unit an option selects, as busybox's free reads it: from the first
// argument alone, -b -k -m or -g, each value rounded to the nearest, and -h each in the
// largest unit that leaves it under 1024, with one decimal, `63.7G`. Kilobytes are the
// default, which is what both references do and why `free` numbers look large. The values
// were cut short rather than rounded, -h was taken and ignored, and of several options the
// first of b, m and g in that order won.
type freeScale struct {
	unit uint64
}

func newFreeApplet() Applet {
	return simpleApplet{name: "free", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "bkmgh", "")
		if err != nil {
			return err
		}
		if len(operands) > 0 {
			return fmt.Errorf("extra operand '%s'", operands[0])
		}
		// The same sampler `top` uses, so the two cannot disagree about how much
		// memory the machine has. Off Windows it refuses with
		// proc.ErrListUnsupported rather than reporting zeros, which is the rule
		// the whole proc package follows: "nothing" and "I cannot see" are
		// different answers and only one is safe to act on.
		snapshot, err := proc.NewSampler().Sample(false)
		if err != nil {
			return err
		}
		return writeFreeTable(stdout, snapshot.Memory, freeScaleFor(options))
	}}
}

func freeScaleFor(options appletOptions) freeScale {
	if len(options.order) == 0 {
		return freeScale{unit: 1024}
	}
	switch options.order[0] {
	case 'b':
		return freeScale{unit: 1}
	case 'm':
		return freeScale{unit: 1 << 20}
	case 'g':
		return freeScale{unit: 1 << 30}
	case 'h':
		return freeScale{unit: 0}
	}
	return freeScale{unit: 1024}
}

func (s freeScale) of(value uint64) string { return humanReadable(value, 1, s.unit) }

// writeFreeTable prints the two rows busybox prints.
//
// The Swap row is Windows' *commit charge*, not a swap file. There is no swap
// partition to measure the way Linux measures one, and commit is the closest
// honest substitute -- it is the number that says whether the next allocation
// will be refused. internal/proc/sample.go:148 records the same reasoning for
// `top`, and the two now agree because they read the same field.
//
// `shared` is always zero: Windows has no equivalent counter, and inventing one
// from working-set overlap would be a guess presented as a measurement.
func writeFreeTable(stdout io.Writer, memory proc.Memory, scale freeScale) error {
	header := fmt.Sprintf("       %12s%12s%12s%12s%12s%12s", "total", "used", "free", "shared", "buff/cache", "available")
	if _, err := fmt.Fprintln(stdout, header); err != nil {
		return err
	}
	mem := fmt.Sprintf("Mem:   %12s%12s%12s%12s%12s%12s",
		scale.of(memory.TotalPhysical),
		scale.of(memory.UsedPhysical()),
		scale.of(memory.AvailablePhysical),
		scale.of(0),
		scale.of(memory.Cached),
		scale.of(memory.AvailablePhysical))
	if _, err := fmt.Fprintln(stdout, mem); err != nil {
		return err
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
	_, err := fmt.Fprintf(stdout, "Swap:  %12s%12s%12s\n",
		scale.of(swapTotal), scale.of(swapUsed), scale.of(swapFree))
	return err
}
