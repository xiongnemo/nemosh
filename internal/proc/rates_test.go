package proc_test

import (
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// A monitor's arithmetic is where the believable-looking mistakes live, and all of them can be
// written down as two structs. None of these tests touches the operating system.

var epoch = time.Date(2026, time.August, 21, 12, 0, 0, 0, time.UTC)

func snapshotAt(offset time.Duration, cpus []proc.CPUTime, processes ...proc.Process) proc.Snapshot {
	return proc.Snapshot{Taken: epoch.Add(offset), CPUs: cpus, Processes: processes}
}

func oneCPU(kernel, user, idle time.Duration) []proc.CPUTime {
	return []proc.CPUTime{{Kernel: kernel, User: user, Idle: idle}}
}

func TestBetween_processCPUIsAShareOfTheWholeMachine(t *testing.T) {
	// Given a process that used one whole second of CPU over one second of wall clock, on a
	// machine with four processors
	four := make([]proc.CPUTime, 4)
	before := snapshotAt(0, four, proc.Process{PID: 7, Created: epoch})
	after := snapshotAt(time.Second, four,
		proc.Process{PID: 7, Created: epoch, User: time.Second})

	// When
	rates := proc.Between(before, after)

	// Then -- a quarter of the machine, which is htop's default reading. One core pinned out
	// of four is 25% and not 100%.
	if got := rates.Processes[7].CPU; got < 0.249 || got > 0.251 {
		t.Fatalf("CPU = %v, want 0.25", got)
	}
}

func TestBetween_ignoresAReusedProcessID(t *testing.T) {
	// Given the same pid in both samples, but a different process wearing it
	before := snapshotAt(0, oneCPU(0, 0, 0),
		proc.Process{PID: 7, Name: "old.exe", Created: epoch, User: 10 * time.Second})
	after := snapshotAt(time.Second, oneCPU(0, 0, 0),
		proc.Process{PID: 7, Name: "new.exe", Created: epoch.Add(time.Millisecond)})

	// When
	rates := proc.Between(before, after)

	// Then -- no rate at all, rather than a wild one from subtracting one program's CPU time
	// from another's. Windows reuses process ids, so this is not a hypothetical.
	if _, ok := rates.Processes[7]; ok {
		t.Fatalf("a reused pid produced a rate: %+v", rates.Processes[7])
	}
}

func TestBetween_refusesToDivideByAnUnusableInterval(t *testing.T) {
	tests := []struct {
		name  string
		later time.Duration
	}{
		{name: "same instant", later: 0},
		{name: "clock stepped backwards", later: -time.Second},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := snapshotAt(0, oneCPU(0, 0, 0), proc.Process{PID: 1, Created: epoch})
			after := snapshotAt(test.later, oneCPU(time.Second, 0, 0),
				proc.Process{PID: 1, Created: epoch, User: time.Second})

			// When
			rates := proc.Between(before, after)

			// Then
			if len(rates.Processes) != 0 || rates.TotalBusy != 0 {
				t.Fatalf("rates from a %v interval: %+v", test.later, rates)
			}
		})
	}
}

func TestBetween_counterGoingBackwardsIsZeroNotNegative(t *testing.T) {
	// Given counters that fell, which a table read while a process exits can look like
	before := snapshotAt(0, oneCPU(0, 0, 0),
		proc.Process{PID: 3, Created: epoch, User: 5 * time.Second, ReadBytes: 1000})
	after := snapshotAt(time.Second, oneCPU(0, 0, 0),
		proc.Process{PID: 3, Created: epoch, User: time.Second, ReadBytes: 10})

	// When
	rate := proc.Between(before, after).Processes[3]

	// Then
	if rate.CPU != 0 || rate.ReadBytesPerSecond != 0 {
		t.Fatalf("rate = %+v, want zeroes rather than negatives", rate)
	}
}

// Windows counts idle time *inside* kernel time. Reading KernelTime as system time shows a
// sleeping machine at full occupancy, which is the single most likely way to get this wrong.
func TestBetween_idleIsInsideKernelTime(t *testing.T) {
	// Given a processor that spent the whole second idle: kernel advanced by a second, and all
	// of it was idle
	before := snapshotAt(0, oneCPU(0, 0, 0))
	after := snapshotAt(time.Second, oneCPU(time.Second, 0, time.Second))

	// When
	rates := proc.Between(before, after)

	// Then
	if rates.CPUs[0].Busy != 0 {
		t.Fatalf("busy = %v on an idle processor", rates.CPUs[0].Busy)
	}
	if rates.TotalBusy != 0 {
		t.Fatalf("total busy = %v on an idle machine", rates.TotalBusy)
	}
}

func TestBetween_perProcessorBreakdown(t *testing.T) {
	// Given one processor: a second of kernel time of which a quarter was idle, plus half a
	// second of user time
	before := snapshotAt(0, oneCPU(0, 0, 0))
	after := snapshotAt(time.Second, []proc.CPUTime{{
		Kernel: time.Second, User: 500 * time.Millisecond,
		Idle: 250 * time.Millisecond, Interrupt: 100 * time.Millisecond,
	}})

	// When
	rate := proc.Between(before, after).CPUs[0]

	// Then -- total accounted is 1.5s, busy is 1.25s of it.
	if rate.Busy < 0.83 || rate.Busy > 0.834 {
		t.Fatalf("busy = %v, want 1.25/1.5", rate.Busy)
	}
	if rate.User < 0.332 || rate.User > 0.334 {
		t.Fatalf("user = %v, want 0.5/1.5", rate.User)
	}
	// System is busy minus user, because idle came out of kernel.
	if rate.Kernel < 0.499 || rate.Kernel > 0.501 {
		t.Fatalf("kernel = %v, want busy minus user", rate.Kernel)
	}
	if rate.Interrupt < 0.066 || rate.Interrupt > 0.067 {
		t.Fatalf("interrupt = %v, want 0.1/1.5", rate.Interrupt)
	}
}

func TestBetween_ioAndFaultRates(t *testing.T) {
	before := snapshotAt(0, oneCPU(0, 0, 0), proc.Process{PID: 9, Created: epoch})
	after := snapshotAt(2*time.Second, oneCPU(0, 0, 0), proc.Process{
		PID: 9, Created: epoch,
		ReadBytes: 2048, WriteBytes: 1024, OtherBytes: 512, HardFaults: 20,
	})

	// When
	rate := proc.Between(before, after).Processes[9]

	// Then -- over two seconds, so half the delta each
	if rate.ReadBytesPerSecond != 1024 || rate.WriteBytesPerSecond != 512 {
		t.Fatalf("io = %+v, want 1024 and 512 per second", rate)
	}
	if rate.OtherBytesPerSecond != 256 || rate.HardFaultsPerSecond != 10 {
		t.Fatalf("other/faults = %+v, want 256 and 10 per second", rate)
	}
}

// A process that has only just started has nothing to compare against, and htop shows 0.0 for
// its first sample too. The alternative -- treating absence as zero-before -- reports a process's
// whole lifetime of CPU as though it were spent in the last second.
func TestBetween_newProcessHasNoRateYet(t *testing.T) {
	before := snapshotAt(0, oneCPU(0, 0, 0))
	after := snapshotAt(time.Second, oneCPU(0, 0, 0),
		proc.Process{PID: 42, Created: epoch, User: time.Hour})

	// When
	rates := proc.Between(before, after)

	// Then
	if _, ok := rates.Processes[42]; ok {
		t.Fatal("a process seen once produced a rate")
	}
}

// The first sample of a run has nothing behind it, and must say so rather than inventing an answer.
//
// This was wrong in a way that could not be seen: the earlier snapshot's Taken is the zero time, so
// the interval came out as roughly two thousand years, the guard against a non-positive interval let
// it through, and every rate divided by it landed on almost exactly zero. The screen showed an idle
// machine. It is the difference between "nothing is happening" and "nothing has been measured", and
// only one of them is true one second into a run.
func TestBetween_withNoEarlierSnapshotThereAreNoRates(t *testing.T) {
	later := proc.Snapshot{
		Taken:     time.Date(2026, time.August, 21, 12, 0, 1, 0, time.UTC),
		Processes: []proc.Process{{PID: 4, Kernel: time.Minute, User: time.Minute}},
		CPUs:      []proc.CPUTime{{Kernel: time.Minute, User: time.Minute}},
	}

	// When
	rates := proc.Between(proc.Snapshot{}, later)

	// Then
	if rates.Interval != 0 {
		t.Fatalf("interval = %v, want zero: there was no earlier sample to measure from", rates.Interval)
	}
	if len(rates.Processes) != 0 || len(rates.CPUs) != 0 || rates.TotalBusy != 0 {
		t.Fatalf("rates were computed from a single sample: %d processes, %d cpus, %v busy",
			len(rates.Processes), len(rates.CPUs), rates.TotalBusy)
	}
}
