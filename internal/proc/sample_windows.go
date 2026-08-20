//go:build windows

package proc

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The whole process table in one call, and no process opened.
//
// This is the finding the monitor rests on. `NtQuerySystemInformation` with
// `SystemProcessInformation` returns every process on the machine -- CPU times, working set,
// thread count, handle count, parent, priority, session, six IO counters, and an inline array of
// one record per thread -- to a caller with no privileges at all.
//
// Measured on an unelevated session, and the contrast is the point: PowerShell's `Get-Process`
// reported CPU time for 249 of 436 processes and an executable path for 176, because it opens
// each process to ask. The same machine's process table answers for all of them, including PID 4.
// **Opening handles is what costs privilege**, and this asks the kernel once instead. That is why
// this can be a monitor an ordinary user runs, where btop++ asks to be elevated.
//
// Two Windows facts shape the code below. The buffer size cannot be known in advance, so the
// call is a grow-and-retry loop, and the buffer is kept between samples because a one-second
// refresh would otherwise churn a megabyte. And `KernelTime` *includes* idle time on the
// per-processor records, which is a trap: subtracting idle is what leaves system time.

const (
	// statusInfoLengthMismatch is STATUS_INFO_LENGTH_MISMATCH: the buffer was too small, and
	// the size needed is written back. It is the documented way to size this call.
	statusInfoLengthMismatch = 0xC0000004
	// hundredNanoseconds is the unit every Windows time in this file uses.
	hundredNanoseconds = 100 * time.Nanosecond
	// initialSampleBuffer is enough for a few hundred processes with their threads. It grows
	// when it has to; starting near the answer avoids the first retry.
	initialSampleBuffer = 1 << 19
)

// systemThreadInformation is SYSTEM_THREAD_INFORMATION, which x/sys/windows does not declare
// even though it declares the process record that is followed by an array of these.
type systemThreadInformation struct {
	KernelTime      int64
	UserTime        int64
	CreateTime      int64
	WaitTime        uint32
	StartAddress    uintptr
	ClientID        struct{ UniqueProcess, UniqueThread uintptr }
	Priority        int32
	BasePriority    int32
	ContextSwitches uint32
	ThreadState     uint32
	WaitReason      uint32
}

// systemProcessorPerformanceInformation is SYSTEM_PROCESSOR_PERFORMANCE_INFORMATION, one per
// logical processor. Also undeclared upstream, though its information class is.
type systemProcessorPerformanceInformation struct {
	IdleTime       int64
	KernelTime     int64
	UserTime       int64
	DpcTime        int64
	InterruptTime  int64
	InterruptCount uint32
}

// Thread states, from the kernel's KTHREAD_STATE. Only the three a monitor acts on are named.
const (
	threadStateRunning  = 2
	threadStateStandby  = 3
	threadStateWaiting  = 5
	waitReasonSuspended = 5
	waitReasonPageIn    = 0
	waitReasonFreePage  = 1
	waitReasonPageOut   = 12
)

// Sampler holds the buffers a repeated sample reuses.
//
// A monitor takes a sample a second for as long as it runs. Allocating the table each time would
// make the thing being measured the biggest allocator on the machine.
type Sampler struct {
	processBuffer   []byte
	processorBuffer []byte
}

// NewSampler returns a sampler with its buffers sized for a typical machine.
func NewSampler() *Sampler {
	return &Sampler{processBuffer: make([]byte, initialSampleBuffer)}
}

// Sample reads the whole system. withThreads asks for the per-thread array to be kept, which the
// caller wants only when a view is expanded to show threads.
func (s *Sampler) Sample(withThreads bool) (Snapshot, error) {
	processes, err := s.processes(withThreads)
	if err != nil {
		return Snapshot{}, err
	}
	cpus, err := s.processors()
	if err != nil {
		return Snapshot{}, err
	}
	memory, err := systemMemory()
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		Taken:     time.Now(),
		Uptime:    systemUptime(),
		Processes: processes,
		CPUs:      cpus,
		Memory:    memory,
	}, nil
}

// query calls NtQuerySystemInformation into buffer, growing it until it fits.
//
// The returned slice aliases the buffer, so the caller must finish with it before the next
// sample. Nothing here escapes past the walk that reads it.
func query(class int32, buffer []byte) ([]byte, []byte, error) {
	if buffer == nil {
		buffer = make([]byte, initialSampleBuffer)
	}
	for attempt := 0; attempt < 8; attempt++ {
		var needed uint32
		status := windows.NtQuerySystemInformation(class, unsafe.Pointer(&buffer[0]),
			uint32(len(buffer)), &needed)
		if status == nil {
			return buffer[:], buffer, nil
		}
		if status != windows.STATUS_INFO_LENGTH_MISMATCH {
			return nil, buffer, fmt.Errorf("querying system information %d: %w", class, status)
		}
		// The kernel says how much it wants; take half again, because the table can grow
		// between the ask and the answer.
		size := int(needed) + int(needed)/2
		if size <= len(buffer) {
			size = len(buffer) * 2
		}
		buffer = make([]byte, size)
	}
	return nil, buffer, fmt.Errorf("querying system information %d: the table kept growing", class)
}
