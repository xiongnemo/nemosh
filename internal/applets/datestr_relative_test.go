package applets

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// date -d and touch -d read GNU's relative items after busybox's formats: `date -d yesterday`,
// `date -d '2 days ago'`, `touch -d 'last monday'`. Each was `invalid date`. The answers are
// GNU's parse_datetime's (parse-datetime.y), over a fixed now: Friday 20 November 2026, 15:30.
func TestParseDateString_relativeItems(t *testing.T) {
	now := time.Date(2026, time.November, 20, 15, 30, 0, 0, time.Local)
	// date -d reads busybox's formats against midnight, as busybox does, and the relative items
	// against now; see date.go.
	midnight := time.Date(2026, time.November, 20, 0, 0, 0, 0, time.Local)
	at := func(month time.Month, day, hour, minute, second int) time.Time {
		return time.Date(2026, month, day, hour, minute, second, 0, time.Local)
	}
	for _, test := range []struct {
		input string
		want  time.Time
	}{
		{"now", now},
		{"today", now},
		{"yesterday", at(time.November, 19, 15, 30, 0)},
		{"Tomorrow", at(time.November, 21, 15, 30, 0)},
		{"2 days ago", at(time.November, 18, 15, 30, 0)},
		{"+3 days", at(time.November, 23, 15, 30, 0)},
		{"-1 week", at(time.November, 13, 15, 30, 0)},
		{"1 fortnight ago", at(time.November, 6, 15, 30, 0)},
		{"3 hours ago", at(time.November, 20, 12, 30, 0)},
		{"90 minutes ago", at(time.November, 20, 14, 0, 0)},
		{"10 seconds", at(time.November, 20, 15, 30, 10)},
		{"10 sec", at(time.November, 20, 15, 30, 10)},
		{"1 month ago", at(time.October, 20, 15, 30, 0)},
		{"last month", at(time.October, 20, 15, 30, 0)},
		{"next week", at(time.November, 27, 15, 30, 0)},
		{"1 year ago", time.Date(2025, time.November, 20, 15, 30, 0, 0, time.Local)},
		// ago negates the item before it, as GNU's grammar has it.
		{"1 day 2 hours ago", at(time.November, 21, 13, 30, 0)},
		// A day of the week is the next such day, or today, at midnight.
		{"monday", at(time.November, 23, 0, 0, 0)},
		{"friday", at(time.November, 20, 0, 0, 0)},
		{"sat", at(time.November, 21, 0, 0, 0)},
		{"next friday", at(time.November, 27, 0, 0, 0)},
		{"NEXT Monday", at(time.November, 23, 0, 0, 0)},
		{"last friday", at(time.November, 13, 0, 0, 0)},
		{"last monday", at(time.November, 16, 0, 0, 0)},
		{"this friday", at(time.November, 20, 0, 0, 0)},
		// With a time of day, which is read as busybox reads it, and then moved.
		{"yesterday 10:00", at(time.November, 19, 10, 0, 0)},
		{"2 days ago 12:00:30", at(time.November, 18, 12, 0, 30)},
		{"monday 09:15", at(time.November, 23, 9, 15, 0)},
	} {
		got, err := parseDateInput(test.input, midnight, now)
		if err != nil || !got.Equal(test.want) {
			t.Errorf("%q: %v, %v; want %v", test.input, got, err, test.want)
		}
	}
	// A date with a relative item is the date, as busybox reads it, moved.
	date, err := parseDateInput("2026-11-01", midnight, now)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := parseDateInput("2026-11-01 +3 days", midnight, now); err != nil || !got.Equal(date.AddDate(0, 0, 3)) {
		t.Errorf("2026-11-01 +3 days: %v, %v; want %v", got, err, date.AddDate(0, 0, 3))
	}
}

// date -d yesterday goes through the applet, against its injected now, and touch -t, a stamp as
// GNU's is, takes no relative item where touch -d does.
func TestDate_readsRelativeItemsThroughTheApplet(t *testing.T) {
	applet := dateApplet{now: func() time.Time { return time.Date(2026, time.November, 20, 15, 30, 0, 0, time.UTC) }}
	var stdout, stderr bytes.Buffer
	if err := applet.Run(context.Background(), []string{"-u", "-d", "yesterday", "+%F %T"}, &bytes.Buffer{}, &stdout, &stderr); err != nil || stdout.String() != "2026-11-19 15:30:00\n" {
		t.Fatalf("date -u -d yesterday: %q, %v (%s)", stdout.String(), err, stderr.String())
	}
	t.Chdir(t.TempDir())
	if _, stderr, status := runApplet(t, "touch", []string{"-d", "2 days ago", "f"}, ""); status != 0 {
		t.Fatalf("touch -d '2 days ago': %d, %s", status, stderr)
	}
	if _, _, status := runApplet(t, "touch", []string{"-t", "yesterday", "f"}, ""); status == 0 {
		t.Fatal("touch -t yesterday took a relative item")
	}
}

func TestParseDateString_refusesWhatIsNoDate(t *testing.T) {
	now := time.Date(2026, time.November, 20, 15, 30, 0, 0, time.Local)
	for _, input := range []string{"3 bananas ago", "next", "ago", "2 days ago ago", "yesterday nonsense", "monday tuesday", ""} {
		if got, err := parseDateInput(input, now, now); err == nil {
			t.Errorf("%q gave %v, want invalid date", input, got)
		}
	}
}
