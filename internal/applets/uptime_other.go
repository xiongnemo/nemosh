//go:build !windows && !linux

package applets

import (
	"encoding/binary"
	"time"

	"golang.org/x/sys/unix"
)

// systemUptime is the BSDs' and macOS's: the boot time sysctl keeps, and vm.loadavg's three
// fixed-point loads over its scale.
func systemUptime() (time.Duration, [3]float64, error) {
	boot, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		return 0, [3]float64{}, err
	}
	up := time.Since(time.Unix(boot.Unix()))
	var loads [3]float64
	if raw, err := unix.SysctlRaw("vm.loadavg"); err == nil && len(raw) >= 16 {
		scale := float64(binary.NativeEndian.Uint64(raw[len(raw)-8:]))
		for index := range loads {
			if scale > 0 {
				loads[index] = float64(binary.NativeEndian.Uint32(raw[index*4:])) / scale
			}
		}
	}
	return up, loads, nil
}
