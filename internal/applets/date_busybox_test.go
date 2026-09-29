package applets

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// date -d reads TIME in every form busybox's parse_datestr does, and -D by a strptime format;
// -R and -I print RFC 2822 and ISO 8601, and %N the nanoseconds. -d took `@SECONDS` alone and
// the rest were refused. The wanted lines are busybox-w32's for the same invocations.
func TestDate_readsAndPrintsAsBusyboxDoes(t *testing.T) {
	applet := dateApplet{now: func() time.Time { return time.Date(2026, time.September, 29, 12, 34, 56, 789000000, time.UTC) }}
	const format = "+%Y-%m-%d_%H:%M:%S"
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"-u", "-d", "2020-01-02", format}, "2020-01-02_00:00:00"},
		{[]string{"-u", "-d", "2020-1-2", format}, "2020-01-02_00:00:00"},
		{[]string{"-u", "-d", "2020-01-02 03:04", format}, "2020-01-02_03:04:00"},
		{[]string{"-u", "-d", "2020-01-02 03:04:05", "+%s"}, "1577934245"},
		{[]string{"-u", "-d", "2020-01-02 03", format}, "2020-01-02_03:00:00"},
		{[]string{"-u", "-d", "2020.01.02-03:04:05", format}, "2020-01-02_03:04:05"},
		{[]string{"-u", "-d", "01.02-03:04", format}, "2026-01-02_03:04:00"},
		{[]string{"-u", "-d", "12:34", format}, "2026-09-29_12:34:00"},
		{[]string{"-u", "-d", "Jan 2 03:04:05 2020", format}, "2020-01-02_03:04:05"},
		{[]string{"-u", "-d", "2020-01-02 03:04 +0100", format}, "2020-01-02_02:04:00"},
		{[]string{"-u", "-d", "202001020304.05", format}, "2020-01-02_03:04:05"},
		{[]string{"-u", "-d", "2001020304", format}, "2020-01-02_03:04:00"},
		{[]string{"-u", "-d", "020304", format}, "2026-09-02_03:04:00"},
		{[]string{"-u", "-d", "2020-02-30", format}, "2020-03-01_00:00:00"},
		{[]string{"-u", "-D", "%d/%m/%Y", "-d", "02/01/2020", format}, "2020-01-02_00:00:00"},
		{[]string{"-u", "--date=@0", "-R"}, "Thu, 01 Jan 1970 00:00:00 +0000"},
		{[]string{"-u", "-d", "@0", "-I"}, "1970-01-01"},
		{[]string{"-u", "-d", "@0", "-Iminutes"}, "1970-01-01T00:00+00:00"},
		{[]string{"-u", "-d", "@0", "-Ins"}, "1970-01-01T00:00:00,000000000+00:00"},
		{[]string{"-u", "+%S.%3N"}, "56.789"},
		{[]string{"-u", "+%N"}, "789000000"},
	} {
		var stdout, stderr bytes.Buffer
		err := applet.Run(context.Background(), test.args, &bytes.Buffer{}, &stdout, &stderr)
		if got := stdout.String(); err != nil || got != test.want+"\n" {
			t.Errorf("date %q: got %q, %q, %v; want %q", test.args, got, stderr.String(), err, test.want)
		}
	}
}
