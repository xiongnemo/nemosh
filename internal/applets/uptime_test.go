package applets

import (
	"regexp"
	"testing"
	"time"
)

// The line is busybox's to the space: days when there are any, hours and minutes, or minutes
// alone in the first hour, and the loads cut to two places, not rounded. Measured against
// busybox-w32's uptime, whose loads on Windows are always 0.00.
func TestUptimeLine_isBusyboxs(t *testing.T) {
	now := time.Date(2026, 10, 5, 6, 26, 42, 0, time.Local)
	for _, test := range []struct {
		up    time.Duration
		loads [3]float64
		want  string
	}{
		{up: 23*24*time.Hour + 7*time.Hour + 15*time.Minute, want: " 06:26:42 up 23 days,  7:15,  load average: 0.00, 0.00, 0.00\n"},
		{up: 24*time.Hour + 3*time.Minute, want: " 06:26:42 up 1 day, 3 min,  load average: 0.00, 0.00, 0.00\n"},
		{up: 59 * time.Minute, loads: [3]float64{1.239, 0.5, 12.999}, want: " 06:26:42 up 59 min,  load average: 1.23, 0.50, 12.99\n"},
		{up: 10*time.Hour + 5*time.Minute, want: " 06:26:42 up 10:05,  load average: 0.00, 0.00, 0.00\n"},
	} {
		if got := uptimeLine(now, test.up, test.loads); got != test.want {
			t.Errorf("uptimeLine(%v) = %q, want %q", test.up, got, test.want)
		}
	}
}

// The applet itself, on this machine, in that shape, and -s the time it started.
func TestUptime_saysHowLongAndSince(t *testing.T) {
	out, stderr, status := runApplet(t, "uptime", nil, "")
	if !regexp.MustCompile(`^ \d\d:\d\d:\d\d up (\d+ days?, )?(\d+ min|[ \d]\d:\d\d),  load average: \d+\.\d\d, \d+\.\d\d, \d+\.\d\d\n$`).MatchString(out) || stderr != "" || status != 0 {
		t.Fatalf("uptime = %q, %q, %d", out, stderr, status)
	}
	if since, _, status := runApplet(t, "uptime", []string{"-s"}, ""); !regexp.MustCompile(`^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\n$`).MatchString(since) || status != 0 {
		t.Fatalf("uptime -s = %q, %d", since, status)
	}
}
