package proc

import "testing"

// The memory arithmetic, which was at 18% and which every meter in `top` and every
// column in `free` is drawn from.
//
// Pure functions over three fields, so they are exactly enumerable -- and each has a
// guard that only a deliberately hostile input reaches, which is precisely the kind
// of branch a spot-check leaves alone.

// UsedPhysical is a *subtraction*, because "used" on Windows is not a counter. The
// guard is what happens when the subtraction would go negative, which cannot happen
// from a healthy sample and would wrap to sixteen exabytes if it did.
func TestMemory_usedPhysicalIsASubtractionThatCannotWrap(t *testing.T) {
	for _, test := range []struct {
		name      string
		total     uint64
		available uint64
		want      uint64
	}{
		{name: "the ordinary case", total: 16 << 30, available: 4 << 30, want: 12 << 30},
		{name: "nothing used", total: 16 << 30, available: 16 << 30, want: 0},
		{name: "everything used", total: 16 << 30, available: 0, want: 16 << 30},
		// More available than total is impossible from a real sample. Answering zero
		// rather than wrapping is the difference between a meter reading empty and a
		// meter reading sixteen exabytes.
		{name: "more available than total", total: 4 << 30, available: 8 << 30, want: 0},
		{name: "an empty sample", total: 0, available: 0, want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			memory := Memory{TotalPhysical: test.total, AvailablePhysical: test.available}
			if got := memory.UsedPhysical(); got != test.want {
				t.Fatalf("UsedPhysical() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestMemory_shareIsAFractionAndNeverDividesByZero(t *testing.T) {
	for _, test := range []struct {
		name      string
		total     uint64
		available uint64
		want      float64
	}{
		{name: "half used", total: 16 << 30, available: 8 << 30, want: 0.5},
		{name: "a quarter used", total: 16 << 30, available: 12 << 30, want: 0.25},
		{name: "nothing used", total: 16 << 30, available: 16 << 30, want: 0},
		{name: "all used", total: 16 << 30, available: 0, want: 1},
		// The first sample of a machine that has not answered yet, and the case a
		// division would panic on.
		{name: "no total at all", total: 0, available: 0, want: 0},
		{name: "no total but some available", total: 0, available: 8 << 30, want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			memory := Memory{TotalPhysical: test.total, AvailablePhysical: test.available}
			if got := memory.Share(); got != test.want {
				t.Fatalf("Share() = %v, want %v", got, test.want)
			}
		})
	}
}

// CommitShare has a meter of its own because it is the number that says whether the
// machine is about to refuse an allocation: commit can be exhausted while physical
// memory still looks free.
func TestMemory_commitShareIsSeparateFromPhysical(t *testing.T) {
	for _, test := range []struct {
		name   string
		commit uint64
		limit  uint64
		want   float64
	}{
		{name: "half promised", commit: 32 << 30, limit: 64 << 30, want: 0.5},
		{name: "nothing promised", commit: 0, limit: 64 << 30, want: 0},
		{name: "the ceiling reached", commit: 64 << 30, limit: 64 << 30, want: 1},
		// Over the limit is possible: Windows can grow the page file, so a sample
		// taken across the growth can read above it. Reported as it is rather than
		// clamped, because a meter over 100% is the true story.
		{name: "past the ceiling", commit: 96 << 30, limit: 64 << 30, want: 1.5},
		{name: "no limit reported", commit: 32 << 30, limit: 0, want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			memory := Memory{CommitTotal: test.commit, CommitLimit: test.limit}
			if got := memory.CommitShare(); got != test.want {
				t.Fatalf("CommitShare() = %v, want %v", got, test.want)
			}
		})
	}
	// The two shares are independent, which is the whole reason both are drawn: a
	// machine can be comfortable on physical memory and out of commit.
	tight := Memory{
		TotalPhysical: 64 << 30, AvailablePhysical: 48 << 30,
		CommitTotal: 63 << 30, CommitLimit: 64 << 30,
	}
	if tight.Share() > 0.3 {
		t.Fatalf("physical share is %v, want the fixture to look comfortable", tight.Share())
	}
	if tight.CommitShare() < 0.9 {
		t.Fatalf("commit share is %v, want the fixture to look tight", tight.CommitShare())
	}
}

// CPUTime's two answers, which differ by the idle time and are easy to confuse.
// Busy is what a percentage is drawn from; Total is what it is a percentage *of*.
func TestCPUTime_busyExcludesIdleAndTotalDoesNot(t *testing.T) {
	sample := CPUTime{Kernel: 300, User: 200, Idle: 100}
	if got := sample.Total(); got != 500 {
		t.Errorf("Total() = %v, want kernel plus user", got)
	}
	if got := sample.Busy(); got != 400 {
		t.Errorf("Busy() = %v, want kernel plus user less idle", got)
	}
	// An idle machine: all of the kernel time was idle, so nothing was busy. Windows
	// counts idle *inside* kernel time, which is the fact these two encode and the
	// one that makes a naive kernel+user percentage read 100% on a quiet machine.
	quiet := CPUTime{Kernel: 1000, User: 0, Idle: 1000}
	if got := quiet.Busy(); got != 0 {
		t.Fatalf("an idle machine reports Busy() = %v, want zero", got)
	}
	if got := quiet.Total(); got != 1000 {
		t.Fatalf("an idle machine reports Total() = %v, want the kernel time", got)
	}
	// A zero sample, which is what the first tick looks like.
	if empty := (CPUTime{}); empty.Busy() != 0 || empty.Total() != 0 {
		t.Fatalf("an empty sample answers %v and %v", empty.Busy(), empty.Total())
	}
}
