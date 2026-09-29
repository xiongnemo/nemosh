package applets

import (
	"fmt"
	"strconv"
)

// du reports how much disk a tree uses.

// duBlock is the unit du counts in. GNU's default is 1024-byte blocks, which is
// why `du -s .` on a 17KB tree says 17 rather than 17408.
//
// The totals are what the filesystem *allocated*, which is what the name means. They used to
// be apparent sizes rounded up to a block, and that was documented as a deliberate
// simplification on the grounds that Go cannot read allocation size portably -- true of
// os.FileInfo, and not true of the platform underneath it. See file_details_windows.go.
const duBlock = 1024

// humanBlocks is GNU's -h: the largest unit that leaves a number under 1024,
// with one decimal below 10. `du -sh` on the tree measured said `17K`.
func humanBlocks(blocks int64) string {
	value := float64(blocks)
	for _, unit := range []string{"K", "M", "G", "T"} {
		if value < 1024 {
			// A whole number keeps its decimal: both references print `4.0K` and this
			// printed `4K`. Above ten the decimal goes, which is GNU's rule; busybox
			// keeps one there too and says `96.7K` where GNU and this say `97K`.
			if value < 10 {
				return fmt.Sprintf("%.1f%s", value, unit)
			}
			return fmt.Sprintf("%d%s", int64(value), unit)
		}
		value /= 1024
	}
	return fmt.Sprintf("%.1fP", value)
}

// humanReadable is busybox's make_human_readable_str: value blocks of blockSize bytes, divided by
// unit and rounded to the nearest, or for a unit of 0 scaled to the largest of K M G T P E with
// one decimal, `4.0K`, and a bare number under 1024.
func humanReadable(value, blockSize, unit uint64) string {
	if value == 0 {
		return "0"
	}
	if blockSize > 1 {
		value *= blockSize
	}
	if unit != 0 {
		return strconv.FormatUint((value+unit/2)/unit, 10)
	}
	suffixes := []string{"", "K", "M", "G", "T", "P", "E"}
	index, fraction := 0, uint64(0)
	for value >= 1024 && index < len(suffixes)-1 {
		index++
		fraction = (value%1024*10 + 1024/2) / 1024
		value /= 1024
	}
	if fraction >= 10 {
		value, fraction = value+1, 0
	}
	if index == 0 {
		return strconv.FormatUint(value, 10)
	}
	return fmt.Sprintf("%d.%d%s", value, fraction, suffixes[index])
}
