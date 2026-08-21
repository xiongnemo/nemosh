//go:build windows

package proc

import (
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Walking the table the kernel just handed over.
//
// The records are variable length -- each carries its threads inline -- so the walk follows
// `NextEntryOffset` rather than indexing an array, and stops on an offset of zero. Nothing is
// copied out of the buffer except the fields wanted, so the buffer can be reused for the next
// sample; see sample_windows.go for why that matters.

// processes reads every process from the system table.
func (s *Sampler) processes(withThreads bool) ([]Process, error) {
	data, buffer, err := query(windows.SystemProcessInformation, s.processBuffer)
	if err != nil {
		return nil, err
	}
	s.processBuffer = buffer
	// About the count a desktop machine runs, so the slice rarely grows.
	processes := make([]Process, 0, 512)
	for offset := 0; offset >= 0 && offset < len(data); {
		record := (*windows.SYSTEM_PROCESS_INFORMATION)(unsafe.Pointer(&data[offset]))
		processes = append(processes, processFromRecord(record, data[offset:], withThreads))
		if record.NextEntryOffset == 0 {
			break
		}
		offset += int(record.NextEntryOffset)
	}
	return processes, nil
}

// processFromRecord converts one record. tail is the buffer from this record onward, which is
// where the thread array lives.
func processFromRecord(record *windows.SYSTEM_PROCESS_INFORMATION, tail []byte, withThreads bool) Process {
	process := Process{
		PID:               int(record.UniqueProcessID),
		PPID:              int(record.InheritedFromUniqueProcessID),
		Name:              processName(record),
		Created:           fromFiletime(record.CreateTime),
		Kernel:            time.Duration(record.KernelTime) * hundredNanoseconds,
		User:              time.Duration(record.UserTime) * hundredNanoseconds,
		Cycles:            record.CycleTime,
		Threads:           int(record.NumberOfThreads),
		Handles:           int(record.HandleCount),
		Session:           int(record.SessionID),
		Priority:          int(record.BasePriority),
		WorkingSet:        uint64(record.WorkingSetSize),
		PrivateWorkingSet: uint64(record.WorkingSetPrivateSize),
		Virtual:           uint64(record.VirtualSize),
		Commit:            uint64(record.PrivatePageCount),
		PageFaults:        uint64(record.PageFaultCount),
		HardFaults:        uint64(record.HardFaultCount),
		ReadOps:           uint64(record.ReadOperationCount),
		WriteOps:          uint64(record.WriteOperationCount),
		OtherOps:          uint64(record.OtherOperationCount),
		ReadBytes:         uint64(record.ReadTransferCount),
		WriteBytes:        uint64(record.WriteTransferCount),
		OtherBytes:        uint64(record.OtherTransferCount),
	}
	threads := threadsFromRecord(record, tail)
	process.State = stateFromThreads(threads)
	if withThreads {
		process.ThreadDetail = threads
	}
	return process
}

// processName is the image file name, or `Idle` for the one process that has none.
//
// PID 0 carries an empty ImageName. Calling it what every other tool calls it beats an empty
// cell, and it is a real row: on an idle machine it holds most of the CPU time.
func processName(record *windows.SYSTEM_PROCESS_INFORMATION) string {
	if record.ImageName.Buffer == nil || record.ImageName.Length == 0 {
		if record.UniqueProcessID == 0 {
			return "Idle"
		}
		return "?"
	}
	return record.ImageName.String()
}

// threadsFromRecord reads the thread array that follows the process record.
//
// fromFiletime converts a Windows FILETIME to a time.
//
// A FILETIME counts hundred-nanosecond ticks from **1601-01-01 UTC**, not from the Unix epoch, and
// this was written as time.Unix(0, ticks*100) -- which put every process's start in 1811. Nothing
// showed it: the three things that read Created all use it relatively, as a cache key, as an
// equality test for pid reuse, and as "is this parent older than its child". All three work
// perfectly with a consistent 369-year offset, so every test passed and every answer was right.
// It would have become wrong the first time a column printed it or compared it against time.Now().
//
// Ticks below the epoch offset are not times at all, and this is not a defensive maybe: **four
// threads on this machine report one**. The System process's first kernel threads -- tids 12, 16, 20
// and 24 -- carry a raw CreateTime of about 1,982,727, a fifth of a second, because they are created
// during boot before the clock is set, so the kernel records a tick count rather than a date. The
// first version of this subtracted the epoch from that, overflowed int64, and reported the year
// 2185. They are reported as unknown instead, which tree.go and the identity checks already handle.
//
// The seconds and nanoseconds are split rather than multiplied out for the same reason: ticks*100
// overflows an int64 two centuries earlier than time.Time needs to.
func fromFiletime(ticks int64) time.Time {
	if ticks < filetimeUnixEpoch {
		return time.Time{}
	}
	ticks -= filetimeUnixEpoch
	return time.Unix(ticks/ticksPerSecond, (ticks%ticksPerSecond)*100).UTC()
}

const (
	// filetimeUnixEpoch is the hundred-nanosecond ticks between 1601-01-01 and 1970-01-01:
	// 11,644,473,600 seconds.
	filetimeUnixEpoch = 116444736000000000
	ticksPerSecond    = 10000000
)

// threadsFromRecord reads the thread array that follows the process record.
//
// This is where Windows is cheaper than Linux rather than dearer: htop opens and reads a
// directory per process to count threads, and here they arrive in the same buffer as the
// process, already filled in.
func threadsFromRecord(record *windows.SYSTEM_PROCESS_INFORMATION, tail []byte) []Thread {
	count := int(record.NumberOfThreads)
	if count <= 0 {
		return nil
	}
	header := int(unsafe.Sizeof(*record))
	size := int(unsafe.Sizeof(systemThreadInformation{}))
	if header+count*size > len(tail) {
		// A record that claims more threads than the buffer holds is a table that changed
		// under the read. Report what fits rather than reading past the end.
		count = (len(tail) - header) / size
		if count <= 0 {
			return nil
		}
	}
	threads := make([]Thread, 0, count)
	for index := 0; index < count; index++ {
		raw := (*systemThreadInformation)(unsafe.Pointer(&tail[header+index*size]))
		threads = append(threads, Thread{
			ID:         int(raw.ClientID.UniqueThread),
			Created:    fromFiletime(raw.CreateTime),
			Kernel:     time.Duration(raw.KernelTime) * hundredNanoseconds,
			User:       time.Duration(raw.UserTime) * hundredNanoseconds,
			Priority:   int(raw.Priority),
			State:      threadState(raw),
			WaitReason: raw.WaitReason,
		})
	}
	return threads
}

// threadState maps a kernel thread state onto the letter a POSIX monitor shows.
func threadState(raw *systemThreadInformation) State {
	switch raw.ThreadState {
	case threadStateRunning, threadStateStandby:
		return StateRunning
	case threadStateWaiting:
		switch raw.WaitReason {
		case waitReasonSuspended:
			return StateStopped
		case waitReasonPageIn, waitReasonFreePage, waitReasonPageOut:
			// Waiting on paging, which is the wait a Linux monitor calls D: not
			// interruptible in any useful sense and not the process's own choice.
			return StateWaiting
		}
		return StateSleeping
	default:
		return StateUnknown
	}
}

// stateFromThreads is the process's state, derived from its threads.
//
// Windows has no process state to read -- this is an approximation, and the rule is the one a
// person would apply looking at the threads: anything running means running, anything stuck in
// a page wait means the process is stuck, all suspended means stopped, otherwise asleep.
//
// There is no zombie, and that is not a gap to fill. A Windows process with no threads left has
// exited; what keeps it in the table is an open handle somewhere, which is a different condition
// from a parent that has not reaped its child.
func stateFromThreads(threads []Thread) State {
	if len(threads) == 0 {
		return StateUnknown
	}
	stopped := true
	waiting := false
	for _, thread := range threads {
		if thread.State == StateRunning {
			return StateRunning
		}
		if thread.State != StateStopped {
			stopped = false
		}
		if thread.State == StateWaiting {
			waiting = true
		}
	}
	switch {
	case stopped:
		return StateStopped
	case waiting:
		return StateWaiting
	default:
		return StateSleeping
	}
}

// processors reads per-logical-processor time.
//
// The buffer has to be an exact multiple of the record size, which is a quirk of the fixed-record
// information classes and cost an hour: the generic grow-and-retry loop doubles the buffer, so it
// never *becomes* a multiple, and the call answers STATUS_INFO_LENGTH_MISMATCH forever. A
// half-megabyte buffer for sixteen small records looked like the safe choice and was the reason
// nothing worked. So this one is sized in whole records from the start.
func (s *Sampler) processors() ([]CPUTime, error) {
	size := int(unsafe.Sizeof(systemProcessorPerformanceInformation{}))
	if len(s.processorBuffer) < size {
		s.processorBuffer = make([]byte, size*max(runtime.NumCPU(), 1))
	}
	var data []byte
	for records := len(s.processorBuffer) / size; records <= 2048; records *= 2 {
		var needed uint32
		buffer := s.processorBuffer[:records*size]
		status := windows.NtQuerySystemInformation(windows.SystemProcessorPerformanceInformation,
			unsafe.Pointer(&buffer[0]), uint32(len(buffer)), &needed)
		if status == nil {
			data = buffer
			break
		}
		if status != windows.STATUS_INFO_LENGTH_MISMATCH {
			return nil, fmt.Errorf("querying processor performance: %w", status)
		}
		s.processorBuffer = make([]byte, records*2*size)
	}
	if data == nil {
		return nil, fmt.Errorf("querying processor performance: no buffer size was accepted")
	}
	count := len(data) / size
	cpus := make([]CPUTime, 0, count)
	for index := 0; index < count; index++ {
		raw := (*systemProcessorPerformanceInformation)(unsafe.Pointer(&data[index*size]))
		if raw.KernelTime == 0 && raw.UserTime == 0 {
			// Past the end of the real processors: the buffer is sized generously and
			// the tail is zeroed.
			break
		}
		cpus = append(cpus, CPUTime{
			Idle:      time.Duration(raw.IdleTime) * hundredNanoseconds,
			Kernel:    time.Duration(raw.KernelTime) * hundredNanoseconds,
			User:      time.Duration(raw.UserTime) * hundredNanoseconds,
			Interrupt: time.Duration(raw.InterruptTime) * hundredNanoseconds,
			DPC:       time.Duration(raw.DpcTime) * hundredNanoseconds,
		})
	}
	return cpus, nil
}
