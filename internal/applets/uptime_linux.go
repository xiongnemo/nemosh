package applets

import (
	"time"

	"golang.org/x/sys/unix"
)

// systemUptime is sysinfo(2)'s, as busybox's reads it: the seconds up and the loads, which are
// fixed-point with sixteen bits of fraction.
func systemUptime() (time.Duration, [3]float64, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0, [3]float64{}, err
	}
	var loads [3]float64
	for index, load := range info.Loads {
		loads[index] = float64(load) / 65536
	}
	return time.Duration(info.Uptime) * time.Second, loads, nil
}
