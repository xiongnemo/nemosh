//go:build windows

package proc

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// What the machine has, from the three calls x/sys/windows does not wrap.
//
// Declared here through NewLazySystemDLL, which is the pattern
// internal/applets/file_details_windows.go already uses for GetDiskFreeSpaceW. None of the three
// needs a privilege: a monitor can report the machine's memory without being trusted with
// anything.

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetTickCount64       = kernel32.NewProc("GetTickCount64")
	psapi                    = windows.NewLazySystemDLL("psapi.dll")
	procGetPerformanceInfo   = psapi.NewProc("GetPerformanceInfo")
)

// memoryStatusEx is MEMORYSTATUSEX. Length must be set before the call, which is how the
// structure is versioned.
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// performanceInformation is PERFORMANCE_INFORMATION, in pages rather than bytes -- which is the
// detail that makes it worth reading alongside MEMORYSTATUSEX rather than instead of it.
type performanceInformation struct {
	Size              uint32
	CommitTotal       uintptr
	CommitLimit       uintptr
	CommitPeak        uintptr
	PhysicalTotal     uintptr
	PhysicalAvailable uintptr
	SystemCache       uintptr
	KernelTotal       uintptr
	KernelPaged       uintptr
	KernelNonpaged    uintptr
	PageSize          uintptr
	HandleCount       uint32
	ProcessCount      uint32
	ThreadCount       uint32
}

// systemMemory reads the machine's memory state.
//
// Both calls, because neither is enough alone: MEMORYSTATUSEX has physical totals and
// PERFORMANCE_INFORMATION has the commit charge, the cache size and the system-wide handle and
// thread counts a monitor's header line wants.
func systemMemory() (Memory, error) {
	status := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	result, _, err := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if result == 0 {
		return Memory{}, fmt.Errorf("reading memory status: %w", err)
	}
	memory := Memory{
		TotalPhysical:     status.TotalPhys,
		AvailablePhysical: status.AvailPhys,
	}
	info := performanceInformation{Size: uint32(unsafe.Sizeof(performanceInformation{}))}
	if result, _, _ := procGetPerformanceInfo.Call(uintptr(unsafe.Pointer(&info)),
		uintptr(info.Size)); result != 0 {
		page := uint64(info.PageSize)
		memory.CommitTotal = uint64(info.CommitTotal) * page
		memory.CommitLimit = uint64(info.CommitLimit) * page
		memory.Cached = uint64(info.SystemCache) * page
		memory.Kernel = uint64(info.KernelTotal) * page
		memory.Handles = int(info.HandleCount)
		memory.Threads = int(info.ThreadCount)
	}
	// A failure of the second call is not fatal: the physical numbers are the ones a header
	// cannot do without, and a missing commit figure is better than no monitor.
	return memory, nil
}

// systemUptime is how long since the machine booted.
//
// GetTickCount64 rather than the boot time from the registry, because it is what every other
// Windows tool reports and it cannot disagree with itself across a clock change.
func systemUptime() time.Duration {
	ticks, _, _ := procGetTickCount64.Call()
	return time.Duration(ticks) * time.Millisecond
}
