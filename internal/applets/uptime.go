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
// here, and Windows has no program of the name.
func newUptimeApplet() Applet {
	return simpleApplet{name: "uptime", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "s", "")
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
		if options.has('s') {
			_, err := fmt.Fprintln(stdout, now.Add(-up.Truncate(time.Second)).Format("2006-01-02 15:04:05"))
			return err
		}
		_, err = io.WriteString(stdout, uptimeLine(now, up, loads))
		return err
	}}
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
