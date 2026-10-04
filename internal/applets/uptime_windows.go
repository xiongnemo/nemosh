package applets

import (
	"time"

	"golang.org/x/sys/windows"
)

// systemUptime is GetTickCount64's milliseconds, as busybox-w32's sysinfo reads them, and
// loads of nothing: Windows keeps no load average.
func systemUptime() (time.Duration, [3]float64, error) {
	return windows.DurationSinceBoot(), [3]float64{}, nil
}
