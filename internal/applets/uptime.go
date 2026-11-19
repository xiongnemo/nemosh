package applets

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// uptime is busybox's: the time now, how long since the machine started, and the load
// averages over one, five and fifteen minutes,
//
//	06:26:42 up 23 days,  7:15,  load average: 0.00, 0.00, 0.00
//
// and with -s the time it started. Windows keeps no load average, and busybox-w32 says 0.00 for
// each, as this does there; nor does it count users, so there is no users field. It was not
// here, and Windows has no program of the name. -p is procps-ng's pretty form, Ubuntu's `uptime
// -p`: up 2 weeks, 3 days, 22 hours, 15 minutes.
func newUptimeApplet() Applet {
	return simpleApplet{name: "uptime", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletLongOptions(ctx, args, "pretty=p since=s", "sp", "")
		if err != nil {
			return err
		}
		if len(operands) > 0 {
			return fmt.Errorf("extra operand '%s'", operands[0])
		}
		up, loads, err := systemUptime()
		if err != nil {
			return err
		}
		now := time.Now()
		// busybox's -s is now less the whole seconds up, which wanders by a second as it does.
		switch options.last("sp") {
		case 's':
			_, err := fmt.Fprintln(stdout, now.Add(-up.Truncate(time.Second)).Format("2006-01-02 15:04:05"))
			return err
		case 'p':
			_, err := fmt.Fprintln(stdout, uptimePretty(up))
			return err
		}
		_, err = io.WriteString(stdout, uptimeLine(now, up, loads))
		return err
	}}
}

// uptimePretty is procps-ng's sprint_uptime with its pretty flag: decades, years within a decade,
// weeks within a year of 52, days within a week, hours and minutes, each when it is not 0, and
// minutes when there are none only in the first minute.
func uptimePretty(up time.Duration) string {
	seconds := int64(up / time.Second)
	parts := []struct {
		count int64
		unit  string
	}{
		{seconds / (86400 * 365 * 10), "decade"},
		{seconds / (86400 * 365) % 10, "year"},
		{seconds / (86400 * 7) % 52, "week"},
		{seconds / 86400 % 7, "day"},
		{seconds / 3600 % 24, "hour"},
		{seconds / 60 % 60, "minute"},
	}
	var said []string
	for index, part := range parts {
		if part.count == 0 && !(index == len(parts)-1 && seconds < 60) {
			continue
		}
		plural := "s"
		if part.count == 1 {
			plural = ""
		}
		said = append(said, fmt.Sprintf("%d %s%s", part.count, part.unit, plural))
	}
	return "up " + strings.Join(said, ", ")
}

// uptimeLine is busybox's line: days when there are any, then hours and minutes, or minutes
// alone in the first hour, and each load average cut, not rounded, to two places.
func uptimeLine(now time.Time, up time.Duration, loads [3]float64) string {
	var line strings.Builder
	fmt.Fprintf(&line, " %02d:%02d:%02d up ", now.Hour(), now.Minute(), now.Second())
	seconds := int64(up / time.Second)
	if days := seconds / 86400; days != 0 {
		plural := "s"
		if days == 1 {
			plural = ""
		}
		fmt.Fprintf(&line, "%d day%s, ", days, plural)
	}
	minutes := seconds / 60
	hours := minutes / 60 % 24
	minutes %= 60
	if hours != 0 {
		fmt.Fprintf(&line, "%2d:%02d", hours, minutes)
	} else {
		fmt.Fprintf(&line, "%d min", minutes)
	}
	line.WriteString(",  load average: ")
	for index, load := range loads {
		if index > 0 {
			line.WriteString(", ")
		}
		hundredths := int64(load * 100)
		fmt.Fprintf(&line, "%d.%02d", hundredths/100, hundredths%100)
	}
	line.WriteString("\n")
	return line.String()
}
