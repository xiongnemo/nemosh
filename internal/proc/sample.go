package proc

import "time"

// A sample is everything a process monitor needs, taken at one instant.
//
// The shape is dictated by how Windows will answer: one call returns the whole table, so a
// sample is naturally a snapshot of everything rather than a walk over per-process files. That
// is the opposite of Linux, where htop opens `/proc/<pid>/stat` once per process and the cost
// scales with the list. Here the cost is one syscall whatever the list looks like.
//
// Everything derived -- CPU percentages, IO rates, the process tree -- is computed from two
// snapshots by the pure functions in rates.go and tree.go, which is what makes any of this
// testable without a machine.

// Snapshot is the whole system at one instant.
type Snapshot struct {
	// Taken is when the sample was read, and it is load-bearing: every rate in this
	// package is a delta divided by the gap between two of these.
	Taken     time.Time
	Uptime    time.Duration
	Processes []Process
	// CPUs is per logical processor, in the order the kernel reports them.
	CPUs   []CPUTime
	Memory Memory
}

// Process is one process as the system table describes it.
//
// Every field here comes from the single unprivileged call described in sample_windows.go. What
// is *not* here -- command line, executable path, owning user, affinity -- needs a handle on the
// process and is therefore best-effort; see detail_windows.go. Keeping the two apart is the
// whole design: the columns that always work are separated from the columns that sometimes do.
type Process struct {
	PID  int
	PPID int
	// Name is the image file name, which is the one description that is always available.
	Name string
	// Created identifies the process together with PID. Windows reuses process ids and does
	// not reparent orphans, so a PID alone identifies nothing across two samples -- see
	// tree.go for what that costs the tree.
	Created time.Time
	// Kernel and User are cumulative CPU consumed. CPU percentage is their delta over the
	// wall-clock gap, divided by the number of processors.
	Kernel time.Duration
	User   time.Duration
	// Cycles is the raw cycle count Windows also keeps. Not used for the percentage --
	// cycles do not divide into wall-clock time on a machine that changes frequency -- but
	// it is the better measure of *relative* consumption between two processes.
	Cycles   uint64
	Threads  int
	Handles  int
	Session  int
	Priority int
	// WorkingSet is resident bytes, the closest thing to RES. PrivateWorkingSet excludes
	// pages shared with other processes and is the honest answer to "what does this process
	// cost me", which is what Task Manager's Memory column shows.
	WorkingSet        uint64
	PrivateWorkingSet uint64
	Virtual           uint64
	// Commit is private committed bytes -- Windows' nearest analogue to swap use, and not
	// the same measure. It counts what has been promised, not what has been paged out.
	Commit     uint64
	PageFaults uint64
	HardFaults uint64
	// ReadBytes, WriteBytes and OtherBytes are cumulative. Their deltas are the IO rates,
	// which on Windows need no privilege at all -- on Linux `/proc/<pid>/io` often does.
	ReadBytes  uint64
	WriteBytes uint64
	OtherBytes uint64
	// State is derived from the process's threads, and is an approximation. Windows has no
	// process state field: what it has is a state and a wait reason per thread. There is no
	// zombie.
	State State
	// ThreadDetail is populated only when a sample was asked for it, because a thousand
	// threads is a thousand structs nobody reads unless the view is expanded.
	ThreadDetail []Thread
}

// Thread is one thread, from the array the system table carries inline after each process.
type Thread struct {
	ID       int
	Kernel   time.Duration
	User     time.Duration
	Priority int
	State    State
	// WaitReason is why a waiting thread is waiting. Windows names about forty reasons; the
	// interesting ones for a monitor are the ones that mean "not going to run soon".
	WaitReason uint32
}

// State is a process or thread state, in the vocabulary a POSIX monitor expects.
type State byte

const (
	// StateUnknown is the honest answer where no thread state could be read.
	StateUnknown State = '?'
	// StateRunning is at least one thread on a processor.
	StateRunning State = 'R'
	// StateSleeping is every thread waiting.
	StateSleeping State = 'S'
	// StateWaiting is every thread in an uninterruptible-looking wait: paging, or a driver
	// call. The nearest thing to Linux's D, and arrived at differently.
	StateWaiting State = 'D'
	// StateStopped is every thread suspended, which is what a debugger leaves behind.
	StateStopped State = 'T'
)

// CPUTime is one logical processor's cumulative time, in the categories Windows keeps.
//
// Richer than `/proc/stat` in one way that matters for a monitor: interrupt and DPC time are
// reported separately, so time the kernel spends in interrupt handlers can be shown as itself
// rather than folded into system time.
type CPUTime struct {
	Idle      time.Duration
	Kernel    time.Duration
	User      time.Duration
	Interrupt time.Duration
	DPC       time.Duration
}

// Busy is everything that is not idle.
//
// Windows reports KernelTime as *including* idle, which is a trap worth stating: subtracting
// idle from kernel is what leaves system time. Getting this wrong shows a machine at 100% while
// it sleeps.
func (c CPUTime) Busy() time.Duration { return c.Kernel + c.User - c.Idle }

// Total is the whole interval this processor accounted for.
func (c CPUTime) Total() time.Duration { return c.Kernel + c.User }

// Memory is what the machine has and what is being used.
type Memory struct {
	TotalPhysical     uint64
	AvailablePhysical uint64
	// CommitTotal and CommitLimit are Windows' commit charge: memory the system has promised
	// and the ceiling on those promises. There is no swap file to measure the way Linux
	// measures one, and this is the closest honest substitute.
	CommitTotal uint64
	CommitLimit uint64
	// Cached is the standby and modified lists -- pages held but reclaimable.
	Cached  uint64
	Kernel  uint64
	Handles int
	Threads int
}

// UsedPhysical is what is not available. Reported rather than computed by callers, because
// "used" on Windows is a subtraction and not a counter.
func (m Memory) UsedPhysical() uint64 {
	if m.AvailablePhysical > m.TotalPhysical {
		return 0
	}
	return m.TotalPhysical - m.AvailablePhysical
}
