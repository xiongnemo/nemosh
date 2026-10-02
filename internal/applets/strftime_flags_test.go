package applets

import (
	"testing"
	"time"
)

// strftime takes glibc's flags and widths, as GNU's date does and busybox's does on Linux. Each
// wanted string is MSYS date's for the same conversion at the epoch, in UTC; busybox-w32's,
// whose strftime is the MSVC runtime's, prints nothing for any of them.
func TestStrftime_takesGlibcsFlagsAndWidths(t *testing.T) {
	epoch := time.Unix(0, 0).UTC()
	for format, want := range map[string]string{
		"%-d": "1", "%_d": " 1", "%0e": "01", "%-m": "1", "%_H": " 0", "%-e": "1", "%e": " 1",
		"%10Y": "0000001970", "%-10d": "1", "%-j": "1", "%_j": "  1", "%-s": "0",
		"%^a": "THU", "%#a": "THU", "%#p": "am", "%#Z": "utc", "%^B": "JANUARY",
		"%_10A": "  Thursday", "%010A": "00Thursday", "%5%": "    %",
		"%q": "1", "%Ey": "70", "%Od": "01", "%d": "01", "%k": " 0",
	} {
		got, err := strftime(epoch, format, true)
		if err != nil || got != want {
			t.Errorf("strftime %q = %q, %v; want %q", format, got, err, want)
		}
	}
	for _, format := range []string{"%-", "%_Q", "%10"} {
		if _, err := strftime(epoch, format, true); err == nil {
			t.Errorf("strftime %q was taken", format)
		}
	}
}
