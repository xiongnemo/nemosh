package applets

import (
	"strconv"
	"time"
)

// The times and shares ps prints, worded as procps-ng's output.c words them, so a script that
// reads `ps -o etime=` or the START of `ps aux` on Linux reads them here too.

// psElapsed is etime, [[DD-]hh:]mm:ss: the hours only once there are any, the days once there are
// any of those.
func psElapsed(elapsed time.Duration) string {
	total := int64(elapsed / time.Second)
	seconds, minutes, hours, days := total%60, total/60%60, total/3600%24, total/86400
	text := ""
	if days > 0 {
		text = strconv.FormatInt(days, 10) + "-"
	}
	if days > 0 || hours > 0 {
		text += twoDigits(hours) + ":"
	}
	return text + twoDigits(minutes) + ":" + twoDigits(seconds)
}

// psCPUTime is time, [DD-]hh:mm:ss, the hours always.
func psCPUTime(used time.Duration) string {
	total := int64(used / time.Second)
	text := ""
	if days := total / 86400; days > 0 {
		text = strconv.FormatInt(days, 10) + "-"
	}
	return text + twoDigits(total/3600%24) + ":" + twoDigits(total/60%60) + ":" + twoDigits(total%60)
}

// psBSDTime is bsdtime, the TIME of `ps aux`: minutes, however many, and seconds.
func psBSDTime(used time.Duration) string {
	total := int64(used / time.Second)
	return strconv.FormatInt(total/60, 10) + ":" + twoDigits(total%60)
}

// psStart is start, STARTED: the time of day within a day of now, and the month and day before.
func psStart(created, now time.Time) string {
	created = created.Local()
	if now.Sub(created) < 24*time.Hour {
		return created.Format("15:04:05")
	}
	return created.Format("Jan 02")
}

// psStartTime is start_time and stime, the START of `ps aux` and the STIME of `ps -ef`: the time
// of day on the same day as now, the month and day in the same year, and otherwise the year.
func psStartTime(created, now time.Time) string {
	created, now = created.Local(), now.Local()
	switch {
	case created.Year() != now.Year():
		return strconv.Itoa(created.Year())
	case created.YearDay() != now.YearDay():
		return created.Format("Jan02")
	}
	return created.Format("15:04")
}

// psBSDStart is bsdstart: the time of day within a day of now, and the month and day, blank-padded
// as ctime pads it, before.
func psBSDStart(created, now time.Time) string {
	created = created.Local()
	if now.Sub(created) <= 24*time.Hour {
		return created.Format("15:04")
	}
	return created.Format("Jan _2")
}

// psLongStart is lstart, the whole of ctime's spelling without its newline.
func psLongStart(created time.Time) string {
	return created.Local().Format("Mon Jan _2 15:04:05 2006")
}

// psPercent is a share in tenths of a percent as %cpu and %mem print it: one decimal, until it is
// past 99.9, and then whole numbers.
func psPercent(tenths int64) string {
	if tenths > 999 {
		return strconv.FormatInt(tenths/10, 10)
	}
	return strconv.FormatInt(tenths/10, 10) + "." + strconv.FormatInt(tenths%10, 10)
}

func twoDigits(value int64) string {
	if value < 10 {
		return "0" + strconv.FormatInt(value, 10)
	}
	return strconv.FormatInt(value, 10)
}
