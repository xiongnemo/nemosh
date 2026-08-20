package applets

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The date column, which had no year in it: a file from 2024 printed `Apr  4 10:00`, so a
// listing of an old directory said nothing about which year anything was from.
//
// Every row was measured against busybox-w32 by giving files crafted timestamps, with the
// clock at 2026-08-20 12:20. `now` is a parameter here so the boundary can be tested without
// waiting six months for it.
func TestLsTimeColumn(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 20, 0, 0, time.UTC)
	tests := []struct {
		name string
		when time.Time
		want string
	}{
		{name: "an hour ago", when: now.Add(-time.Hour), want: "Aug 20 11:20"},
		{name: "this month", when: time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC), want: "Aug 05 10:00"},
		{name: "three months back", when: time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC), want: "May 01 10:00"},
		// Six months and a day: the year takes over. busybox printed `Feb 01  2026` for
		// this one where it printed `May 01 10:00` for the row above.
		{name: "just over six months", when: time.Date(2026, time.February, 1, 10, 0, 0, 0, time.UTC), want: "Feb 01  2026"},
		{name: "years back", when: time.Date(2024, time.April, 4, 10, 0, 0, 0, time.UTC), want: "Apr 04  2024"},
		// More than an hour ahead is suspect enough that the year is the more useful
		// thing to print. GNU's rule, and busybox follows it: a file stamped 13:30 with
		// the clock at 12:20 came out as `Aug 20  2026`.
		{name: "an hour and ten ahead", when: now.Add(70 * time.Minute), want: "Aug 20  2026"},
		{name: "half an hour ahead", when: now.Add(30 * time.Minute), want: "Aug 20 12:50"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := lsTimeColumn(test.when, now)

			// Then
			if got != test.want {
				t.Fatalf("lsTimeColumn(%s) = %q, want %q", test.when.Format(time.RFC3339), got, test.want)
			}
		})
	}
}

// The two date forms have to be the same width or the name column moves between rows, which is
// the whole reason the year form carries two spaces.
func TestLsTimeColumn_bothFormsAreTwelveWide(t *testing.T) {
	now := time.Date(2026, time.August, 20, 12, 20, 0, 0, time.UTC)
	for _, when := range []time.Time{now, now.AddDate(-2, 0, 0)} {
		// When
		got := lsTimeColumn(when, now)

		// Then
		if len(got) != 12 {
			t.Fatalf("lsTimeColumn(%s) = %q, which is %d wide, want 12",
				when.Format(time.RFC3339), got, len(got))
		}
	}
}

// A link says so in the first column and names its target. On Windows the entries that matter
// are *junctions* rather than symlinks -- there are ten in a home directory -- and Go reports
// one as ModeIrregular, so `ls -l` printed `?rw-rw-rw-` with a size of 0 and no target at all.
func TestFormatLongEntry_namesALinkTarget(t *testing.T) {
	// Given
	directory := t.TempDir()
	target := filepath.Join(directory, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		// Creating one needs a privilege an ordinary Windows session has not got unless
		// developer mode is on. Skipping is right: the junctions this exists for are
		// made by Windows itself, and there is no way to make one here.
		t.Skipf("cannot create a symlink: %v", err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}

	// When
	line := formatLongEntry(link, "link.txt", info, "0")

	// Then
	if line[0] != 'l' {
		t.Fatalf("line = %q, want it to start with l", line)
	}
	slashed := filepath.ToSlash(target)
	if want := "link.txt -> " + slashed; !strings.HasSuffix(line, want) {
		t.Fatalf("line = %q, want it to end with %q", line, want)
	}
	// A link's size is the length of its target, which is POSIX and what busybox prints.
	if fields := strings.Fields(line); fields[4] != strconv.Itoa(len(slashed)) {
		t.Fatalf("size field = %q, want the target's length %d", fields[4], len(slashed))
	}
}
