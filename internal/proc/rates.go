package proc

import "time"

// Turning two snapshots into rates.
//
// Nothing here touches the operating system, which is the point: a monitor's arithmetic is where
// the believable-looking mistakes live -- a percentage over the wrong denominator, a counter that
// went backwards, a first sample with nothing to compare against -- and all of them can be
// written down as a test over two structs.
//
// The denominator is the one worth stating. A process's CPU percentage is its own CPU delta over
// the *wall-clock* gap, and then over the processor count, so a process pinning one core of eight
// reads 12.5% rather than 100%. That is htop's default, and htop can be told to report the other
// convention with its `-H` irix flag; this reports the first and says so.

// Rates is what changed between two snapshots.
type Rates struct {
	// Interval is the wall-clock gap the deltas were taken over. Zero means there was no
	// usable pair, and every rate is zero rather than infinite.
	Interval time.Duration
	// Processes is keyed by pid, holding only processes present in both snapshots. A process
	// that has just started has no delta yet, which is honest: htop shows 0.0 for its first
	// sample too.
	Processes map[int]ProcessRate
	// CPUs is per logical processor, in the same order as the snapshot.
	CPUs []CPURate
	// TotalBusy is the machine as a whole, which is not the mean of CPUs when a processor
	// was added or removed between samples.
	TotalBusy float64
}

// ProcessRate is one process's movement between two samples.
type ProcessRate struct {
	// CPU is a fraction of one machine: 1.0 means every processor fully occupied.
	CPU float64
	// ReadBytesPerSecond and the two beside it are IO throughput. Windows gives these to an
	// unprivileged caller, which Linux frequently does not.
	ReadBytesPerSecond  float64
	WriteBytesPerSecond float64
	OtherBytesPerSecond float64
	// HardFaultsPerSecond is page-ins from disk -- the number that tells you a machine is
	// thrashing rather than merely busy.
	HardFaultsPerSecond float64
}

// CPURate is one processor's occupancy, each field a fraction of that processor.
type CPURate struct {
	Busy      float64
	User      float64
	Kernel    float64
	Interrupt float64
	DPC       float64
}

// Between computes the rates from earlier to later.
//
// A zero or negative interval yields empty rates rather than dividing by it, which happens more
// often than it sounds: two samples inside one clock tick, or a clock that stepped backwards.
func Between(earlier, later Snapshot) Rates {
	interval := later.Taken.Sub(earlier.Taken)
	rates := Rates{Interval: interval, Processes: map[int]ProcessRate{}}
	if interval <= 0 {
		return rates
	}
	seconds := interval.Seconds()
	processors := len(later.CPUs)
	if processors == 0 {
		processors = 1
	}
	previous := make(map[int]Process, len(earlier.Processes))
	for _, process := range earlier.Processes {
		previous[process.PID] = process
	}
	for _, process := range later.Processes {
		before, ok := previous[process.PID]
		// Identity is pid *and* start time. Windows reuses process ids, so without this a
		// short-lived process replaced by another wearing its number would show a wild
		// percentage from subtracting one program's CPU time from another's.
		if !ok || !before.Created.Equal(process.Created) {
			continue
		}
		rates.Processes[process.PID] = ProcessRate{
			CPU:                 cpuFraction(before, process, interval, processors),
			ReadBytesPerSecond:  perSecond(before.ReadBytes, process.ReadBytes, seconds),
			WriteBytesPerSecond: perSecond(before.WriteBytes, process.WriteBytes, seconds),
			OtherBytesPerSecond: perSecond(before.OtherBytes, process.OtherBytes, seconds),
			HardFaultsPerSecond: perSecond(before.HardFaults, process.HardFaults, seconds),
		}
	}
	rates.CPUs, rates.TotalBusy = cpuRates(earlier.CPUs, later.CPUs)
	return rates
}

// cpuFraction is a process's share of the whole machine.
func cpuFraction(before, after Process, interval time.Duration, processors int) float64 {
	used := (after.Kernel + after.User) - (before.Kernel + before.User)
	if used <= 0 {
		// Counters do not run backwards, but a table read while a process is exiting can
		// look as though they did. Zero beats a negative percentage.
		return 0
	}
	fraction := float64(used) / float64(interval) / float64(processors)
	return clampFraction(fraction)
}

// perSecond is a counter delta over the interval, in units per second.
func perSecond(before, after uint64, seconds float64) float64 {
	if after <= before || seconds <= 0 {
		return 0
	}
	return float64(after-before) / seconds
}

// cpuRates is per-processor occupancy and the machine's total.
//
// Each processor's own reported time is the denominator rather than the wall clock, because that
// is what makes the arithmetic robust: a processor parked by the kernel accounts for less time
// than the interval, and dividing by the interval would show it as idle when it was absent.
func cpuRates(earlier, later []CPUTime) ([]CPURate, float64) {
	count := min(len(earlier), len(later))
	rates := make([]CPURate, 0, count)
	var busySum, totalSum float64
	for index := 0; index < count; index++ {
		before, after := earlier[index], later[index]
		total := float64(after.Total() - before.Total())
		if total <= 0 {
			rates = append(rates, CPURate{})
			continue
		}
		busy := float64(after.Busy() - before.Busy())
		rate := CPURate{
			Busy:      clampFraction(busy / total),
			User:      clampFraction(float64(after.User-before.User) / total),
			Interrupt: clampFraction(float64(after.Interrupt-before.Interrupt) / total),
			DPC:       clampFraction(float64(after.DPC-before.DPC) / total),
		}
		// System time is kernel *minus* idle, because Windows counts idle inside kernel.
		// Reading KernelTime as system time shows an asleep machine at full occupancy.
		rate.Kernel = clampFraction(rate.Busy - rate.User)
		rates = append(rates, rate)
		busySum += busy
		totalSum += total
	}
	if totalSum <= 0 {
		return rates, 0
	}
	return rates, clampFraction(busySum / totalSum)
}

// clampFraction keeps a fraction inside 0..1.
//
// Rounding across two counters read at slightly different moments can put a busy processor a
// hair over 1.0, and a meter drawn from 1.02 overflows its bar.
func clampFraction(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}
