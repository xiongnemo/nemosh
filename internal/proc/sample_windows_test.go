//go:build windows

package proc_test

import (
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// The claim this whole package rests on, asserted against the running machine: the system table
// answers for *every* process without a handle being opened, including ones this session has no
// right to open.
//
// Measured while writing it, unelevated: PowerShell's Get-Process reported CPU time for 249 of
// 436 processes and a path for 176, because it opens each one. If this test ever starts failing
// on the SYSTEM processes, the sampler has quietly started opening handles and the monitor has
// stopped working for the user it was built for.
func TestSample_seesEveryProcessUnprivileged(t *testing.T) {
	sampler := proc.NewSampler()

	// When
	snapshot, err := sampler.Sample(false)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}

	// Then
	if len(snapshot.Processes) < 20 {
		t.Fatalf("saw %d processes, want a plausible machine", len(snapshot.Processes))
	}
	if len(snapshot.CPUs) == 0 {
		t.Fatal("no processors reported")
	}
	if snapshot.Memory.TotalPhysical == 0 {
		t.Fatal("no physical memory reported")
	}
	if snapshot.Uptime <= 0 {
		t.Fatalf("uptime = %v, want a positive duration", snapshot.Uptime)
	}

	// PID 4 is the kernel. It is the process this session is least entitled to open, so it is
	// the one worth naming: CPU time and a thread count for it prove the data did not come
	// through a handle.
	var system proc.Process
	for _, process := range snapshot.Processes {
		if process.PID == 4 {
			system = process
		}
	}
	if system.PID != 4 {
		t.Fatal("PID 4 (System) is missing from the table")
	}
	if system.Kernel+system.User <= 0 {
		t.Fatal("PID 4 has no CPU time; the sampler is not reading the system table")
	}
	if system.Threads <= 0 {
		t.Fatal("PID 4 has no threads")
	}
}

// Every process must have the fields a monitor cannot draw a row without.
func TestSample_everyRowIsUsable(t *testing.T) {
	snapshot, err := proc.NewSampler().Sample(false)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	for _, process := range snapshot.Processes {
		if process.Name == "" {
			t.Fatalf("pid %d has no name", process.PID)
		}
		if process.PID != 0 && process.Created.IsZero() {
			// PID 0 has no creation time; everything else must, because identity
			// across two samples depends on it.
			t.Fatalf("pid %d (%s) has no creation time", process.PID, process.Name)
		}
	}
}

// Threads arrive in the same buffer as their process, which is the one place Windows is cheaper
// than Linux here: htop reads a directory per process to count them.
func TestSample_threadDetailOnRequest(t *testing.T) {
	sampler := proc.NewSampler()

	// When
	without, err := sampler.Sample(false)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	with, err := sampler.Sample(true)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}

	// Then
	for _, process := range without.Processes {
		if len(process.ThreadDetail) != 0 {
			t.Fatalf("pid %d carried thread detail that was not asked for", process.PID)
		}
	}
	found := false
	for _, process := range with.Processes {
		if len(process.ThreadDetail) > 0 {
			found = true
			if len(process.ThreadDetail) != process.Threads {
				t.Fatalf("pid %d reports %d threads and carried %d",
					process.PID, process.Threads, len(process.ThreadDetail))
			}
		}
	}
	if !found {
		t.Fatal("no process carried thread detail when it was asked for")
	}
}

// Two real samples a moment apart, to catch the arithmetic being wrong about the machine it is
// actually running on -- a percentage over 100, or a busy machine reading as idle.
func TestBetween_onTheRunningMachine(t *testing.T) {
	sampler := proc.NewSampler()
	first, err := sampler.Sample(false)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	// Long enough that the kernel's 15.6 ms tick has moved, short enough not to slow the
	// suite down.
	time.Sleep(120 * time.Millisecond)
	second, err := sampler.Sample(false)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}

	// When
	rates := proc.Between(first, second)

	// Then
	if rates.Interval <= 0 {
		t.Fatalf("interval = %v", rates.Interval)
	}
	if len(rates.Processes) == 0 {
		t.Fatal("no process appeared in both samples")
	}
	if rates.TotalBusy < 0 || rates.TotalBusy > 1 {
		t.Fatalf("total busy = %v, want a fraction", rates.TotalBusy)
	}
	for pid, rate := range rates.Processes {
		if rate.CPU < 0 || rate.CPU > 1 {
			t.Fatalf("pid %d cpu = %v, want a fraction of one machine", pid, rate.CPU)
		}
	}
	for index, cpu := range rates.CPUs {
		if cpu.Busy < 0 || cpu.Busy > 1 {
			t.Fatalf("cpu %d busy = %v, want a fraction", index, cpu.Busy)
		}
	}
}
