package applets

import (
	"fmt"
	"strings"
	"time"
)

// GNU's --time-style, which busybox's ls has not got and which Debian users set so that a listing
// reads the same on every machine: full-iso, the time to the nanosecond and its zone; long-iso,
// to the minute; iso, the month, day and time for a recent file and the date for an old one, both
// eleven wide; locale, the usual column; and +FORMAT, strftime's, with a newline parting the
// format for an old file from the one for a recent file. posix- before a style means it, as GNU
// reads it outside the POSIX locale, which this has no other of.

// lsTimeStyle is a style as it is written, or GNU's refusal.
func lsTimeStyle(value string) (string, error) {
	style := strings.TrimPrefix(value, "posix-")
	switch {
	case strings.HasPrefix(value, "+"):
		return value, nil
	case style == "full-iso", style == "long-iso", style == "iso", style == "locale":
		return style, nil
	}
	return "", fmt.Errorf("invalid argument '%s' for '--time-style'", value)
}

// lsTimeText is a long listing's time: --time-style's, then --full-time's, which is busybox's and
// has no fraction, and otherwise the usual column.
func lsTimeText(when, now time.Time, options lsOptions) string {
	recent := !when.After(now.Add(lsFutureAllowance)) && !when.Before(now.AddDate(0, lsRecentWindowMonths, 0))
	switch style := options.timeStyle; {
	case style == "full-iso":
		return when.Format("2006-01-02 15:04:05.000000000 -0700")
	case style == "long-iso":
		return when.Format("2006-01-02 15:04")
	case style == "iso" && recent:
		return when.Format("01-02 15:04")
	case style == "iso":
		return when.Format("2006-01-02 ")
	case strings.HasPrefix(style, "+"):
		format := style[1:]
		if old, fresh, parted := strings.Cut(format, "\n"); parted && recent {
			format = fresh
		} else {
			format = old
		}
		text, _ := strftime(when, format, false)
		return text
	case options.fullTime && style == "":
		return when.Format("2006-01-02 15:04:05 -0700")
	}
	return lsTimeColumn(when, now)
}
