//go:build windows

package runtime

import (
	"syscall"
	"testing"
	"time"
)

// The kernel and user FILETIMEs GetProcessTimes reports are durations in
// 100-nanosecond intervals, not instants. syscall.Filetime.Nanoseconds is for
// the instant form: it subtracts the 1601-to-1970 offset first, which sends a
// small duration far negative and then past the int64 floor, so a shell that
// has used no measurable CPU reports roughly 215 years of it.
func TestFiletimeDuration_readsAFiletimeAsADurationNotAnInstant(t *testing.T) {
	for _, test := range []struct {
		name      string
		intervals int64
		want      time.Duration
	}{
		{name: "no cpu used at all", intervals: 0, want: 0},
		{name: "one scheduler quantum", intervals: 156250, want: 15625 * time.Microsecond},
		{name: "one second", intervals: 10_000_000, want: time.Second},
		{name: "wide enough to use the high word", intervals: 1 << 33, want: time.Duration(1<<33) * 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			filetime := syscall.Filetime{
				LowDateTime:  uint32(test.intervals & 0xFFFFFFFF),
				HighDateTime: uint32(test.intervals >> 32),
			}

			// When
			got := filetimeDuration(filetime)

			// Then
			if got != test.want {
				t.Fatalf("filetimeDuration(%d intervals) = %v, want %v", test.intervals, got, test.want)
			}
		})
	}
}
