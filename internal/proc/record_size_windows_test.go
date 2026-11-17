package proc

import (
	"testing"
	"unsafe"
)

// A thread's record and a processor's hold a LARGE_INTEGER, which C aligns to 8 on every Windows
// and Go aligns to 4 on windows/386, so Go's declaration of them ends 4 bytes short there: 60 for
// a thread where Windows lays 64 between one and the next, 44 for a processor where it lays 48.
// Reading the second thread at the first's size gave `top` start times in the year 30188, and a
// buffer of 44-byte processor records is no length the query accepts. The sizes here are the
// ones SDK headers give: x86, then x64 and ARM64, which agree.
func TestRecordSize_isTheSizeCGivesTheRecord(t *testing.T) {
	thread, processor := 80, 48
	if unsafe.Sizeof(uintptr(0)) == 4 {
		thread = 64
	}
	if got := recordSize(unsafe.Sizeof(systemThreadInformation{})); got != thread {
		t.Errorf("SYSTEM_THREAD_INFORMATION is laid out every %d bytes, want %d", got, thread)
	}
	if got := recordSize(unsafe.Sizeof(systemProcessorPerformanceInformation{})); got != processor {
		t.Errorf("SYSTEM_PROCESSOR_PERFORMANCE_INFORMATION is laid out every %d bytes, want %d", got, processor)
	}
}
