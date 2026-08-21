package applets

import (
	"fmt"
	"time"
)

// How a monitor writes a number, which is not how anything else here writes one.
//
// A table refreshing every second is read by glancing, so every value has to occupy a fixed
// width and carry its unit -- a column that shifts between `999K` and `1.0M` is harder to scan
// than one that is always five characters. These are separate from du's and ls's human sizes for
// that reason: those pad to the widest value they happen to have, and a monitor cannot wait to
// see what its widest value will be.

// topBytes is a size in at most five characters: `1023B`, `9.9K`, `123M`, `1.5G`.
//
// One decimal below ten and none above, which is the rule ls and du already follow here, so the
// three do not disagree about what 1536 bytes looks like.
func topBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%dB", value)
	}
	size := float64(value)
	for _, suffix := range []string{"K", "M", "G", "T", "P"} {
		size /= unit
		if size < unit {
			if size < 10 {
				return fmt.Sprintf("%.1f%s", size, suffix)
			}
			return fmt.Sprintf("%.0f%s", size, suffix)
		}
	}
	return fmt.Sprintf("%.0fP", size)
}

// topRate is bytes per second, with the same shape and a trailing `/s` left to the header so the
// cell stays narrow.
func topRate(value float64) string {
	if value < 1 {
		// Not `0.0B`: a resting process should read as resting, and a dash is quieter
		// than a zero in a column that is mostly zero.
		return "-"
	}
	return topBytes(uint64(value))
}

// topPercent is a percentage in five characters, `  0.0` to `100.0`.
func topPercent(fraction float64) string {
	return fmt.Sprintf("%5.1f", fraction*100)
}

// topCPUTime is cumulative CPU as htop writes it: `MMM:SS.cc`, minutes and hundredths.
//
// Minutes rather than hours, because that is what top and htop both print and what anyone reading
// the column expects -- a process with two days of CPU shows as `2880:00.00`, which is odd-looking
// and correct, and is how every other monitor says it.
//
// Past ten thousand minutes it switches to hours and then to days, which htop never has to do and
// this platform does. Windows has an Idle process, its CPU time is every idle moment on every
// processor, and on this machine that is `264482:43.25` -- twelve characters in a column sized for
// nine. The choice was between a column three cells wider than anything else needs, cutting digits
// off a number, and changing the unit. Only the last of those is both aligned and true.
func topCPUTime(used time.Duration) string {
	minutes := int64(used / time.Minute)
	if minutes < 10000 {
		seconds := int64(used/time.Second) % 60
		hundredths := int64(used/(10*time.Millisecond)) % 100
		return fmt.Sprintf("%d:%02d.%02d", minutes, seconds, hundredths)
	}
	hours := int64(used / time.Hour)
	if hours < 10000 {
		return fmt.Sprintf("%dh%02dm", hours, minutes%60)
	}
	return fmt.Sprintf("%dd%02dh", int64(used/(24*time.Hour)), hours%24)
}

// topUptime is how long the machine has been up, in the shape uptime(1) uses.
func topUptime(since time.Duration) string {
	days := int64(since / (24 * time.Hour))
	hours := int64(since/time.Hour) % 24
	minutes := int64(since/time.Minute) % 60
	if days > 0 {
		return fmt.Sprintf("%d days, %02d:%02d", days, hours, minutes)
	}
	return fmt.Sprintf("%02d:%02d", hours, minutes)
}

// topPriority names a Windows base priority the way a person would recognise it.
//
// Windows has priority *classes* where POSIX has a nice value from -20 to 19, and the mapping is
// coarse in one direction only: these six names cover every class a process can be in, and no
// arithmetic on them produces the others. Showing the number alone would be honest and useless --
// 8 means nothing to a reader -- and showing a fabricated nice value would be worse.
func topPriority(base int) string {
	switch {
	case base >= 24:
		return "real"
	case base >= 13:
		return "high"
	case base >= 10:
		return "above"
	case base >= 8:
		return "norm"
	case base >= 6:
		return "below"
	default:
		return "idle"
	}
}
