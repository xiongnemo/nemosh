package proc

import (
	"os"
	"testing"
	"time"
)

// List and the detail cache, both of which were at 0%.
//
// They are the foundation `top`, `ps` and `free` are built on, and they read the real
// process table -- so the assertions are about *shape and identity*, never about a
// number. A process count, a memory figure and a thread count all move between one
// call and the next, and a test that pinned one would fail on a busy machine rather
// than on a broken one.
//
// The exactly assertable facts turn out to be about what List *omits*, which is the
// more interesting property anyway -- see the test below.

// List excludes the calling process, and that is deliberate rather than a gap.
//
// It is pgrep and pkill's view of the machine, and leaving the caller out is what
// stops `pkill sh` from killing the shell that ran it. The Sampler underneath does
// include it, because `top` wants every process including its own -- so the two
// views differ by exactly one row and this test is what says the difference is on
// purpose. The first draft asserted the opposite and was wrong.
func TestList_omitsTheCallerAndTheIdleProcess(t *testing.T) {
	processes, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	self := os.Getpid()
	for _, process := range processes {
		if process.PID == self {
			t.Fatalf("List included the calling process (pid %d); pkill would kill its own shell", self)
		}
		if process.PID == 0 {
			t.Fatal("List included pid 0, which is not a process anything may signal")
		}
	}

	// And the sampler *does* include the caller, which is what makes the exclusion a
	// choice rather than a limitation of the table underneath.
	snapshot, err := NewSampler().Sample(false)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	found := false
	for _, process := range snapshot.Processes {
		if process.PID == self {
			found = true
		}
	}
	if !found {
		t.Fatalf("the sampler does not see this process either, so List is not choosing to omit it")
	}
	// One row apart, give or take whatever started or exited between the two calls.
	// Asserted loosely on purpose: a tight count would fail on a busy machine rather
	// than on a broken one.
	if len(snapshot.Processes) <= len(processes) {
		t.Fatalf("the sampler saw %d processes and List saw %d; List should see fewer",
			len(snapshot.Processes), len(processes))
	}
}

func TestList_describesEveryRowUsably(t *testing.T) {
	processes, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(processes) < 2 {
		t.Fatalf("the machine reported %d processes, which cannot be right", len(processes))
	}

	seen := map[int]bool{}
	for _, process := range processes {
		if seen[process.PID] {
			t.Errorf("pid %d appears twice in one listing", process.PID)
		}
		seen[process.PID] = true
		// Every row has to be usable by the columns that read it, whatever process
		// it describes. Nothing here is a *number* to compare against: a count or a
		// size moves between two calls, so what is checked is that each field could
		// be rendered rather than what it says.
		if process.PID < 0 {
			t.Errorf("a process reports pid %d", process.PID)
		}
		if process.Name == "" {
			t.Errorf("pid %d has no name, so the COMMAND column would be blank", process.PID)
		}
		if process.Threads < 0 {
			t.Errorf("pid %d reports %d threads", process.PID, process.Threads)
		}
		// Created is what distinguishes a reissued pid from the process that had it
		// before, so a zero here would make the detail cache describe one process
		// with another's command line.
		if process.Created.IsZero() && process.PID > 4 {
			t.Errorf("pid %d has no creation time", process.PID)
		}
		if process.Created.After(time.Now().Add(time.Minute)) {
			t.Errorf("pid %d claims to have started in the future", process.PID)
		}
	}
}

// The cache asks the system once per process and remembers the refusals too, which is
// what keeps a monitor redrawing at one hertz from re-opening every process on the
// machine every second.
func TestDetailCache_answersTheSameThingTwice(t *testing.T) {
	// From the sampler rather than from List, because List omits the caller on
	// purpose -- and this process is the one whose details are certain to be
	// readable, since a process may always open itself.
	snapshot, err := NewSampler().Sample(false)
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	self := os.Getpid()
	var subject Process
	for _, process := range snapshot.Processes {
		if process.PID == self {
			subject = process
		}
	}
	if subject.PID != self {
		t.Fatalf("the sampler does not report this process")
	}
	// What the sampler says about *this* process can be checked, because it is known.
	if subject.Threads < 1 {
		t.Errorf("this process reports %d threads", subject.Threads)
	}
	if subject.WorkingSet == 0 {
		t.Errorf("this process reports no working set")
	}
	if subject.Created.After(time.Now()) {
		t.Errorf("this process started in the future: %v", subject.Created)
	}

	cache := NewDetailCache()
	first := cache.Lookup(subject)
	second := cache.Lookup(subject)
	if first != second {
		t.Fatalf("two lookups of one process disagreed:\n  %+v\n  %+v", first, second)
	}
	// This process can always be opened by itself, so its own details are known
	// rather than refused -- which is what makes them assertable at all.
	if first.CommandLine == "" && first.Path == "" {
		t.Fatalf("neither a command line nor a path for this process: %+v", first)
	}
}

// The key is pid *and* creation time, because a pid alone is not an identity on
// Windows: the number is reissued. A cache keyed on the pid alone would answer a new
// process with a dead one's command line, which is the defect this design avoids and
// therefore the one worth a test.
func TestDetailCache_treatsAReissuedPidAsADifferentProcess(t *testing.T) {
	cache := NewDetailCache()
	created := time.Now()
	original := Process{PID: 4242, Name: "first.exe", Created: created}
	reissued := Process{PID: 4242, Name: "second.exe", Created: created.Add(time.Second)}

	// Neither can be opened -- pid 4242 is almost certainly not this machine's --
	// so both answer empty details. What is asserted is that the cache stored *two*
	// entries and not one, which is the identity question rather than the contents.
	cache.Lookup(original)
	cache.Lookup(reissued)
	cache.mu.Lock()
	entries := len(cache.entries)
	cache.mu.Unlock()
	if entries != 2 {
		t.Fatalf("the cache holds %d entries for one pid at two creation times, want 2", entries)
	}
}

// Forget is what stops a long-running monitor from remembering every process the
// machine has ever run.
func TestDetailCache_forgetsWhatIsNoLongerRunning(t *testing.T) {
	cache := NewDetailCache()
	now := time.Now()
	for _, pid := range []int{101, 102, 103} {
		cache.Lookup(Process{PID: pid, Name: "x.exe", Created: now})
	}
	cache.mu.Lock()
	before := len(cache.entries)
	cache.mu.Unlock()
	if before != 3 {
		t.Fatalf("the cache holds %d entries, want 3", before)
	}

	// Only 102 is still alive.
	cache.Forget(map[int]bool{102: true})
	cache.mu.Lock()
	after := len(cache.entries)
	_, kept := cache.entries[detailKey{pid: 102, created: now.UnixNano()}]
	cache.mu.Unlock()
	if after != 1 {
		t.Fatalf("the cache holds %d entries after forgetting, want 1", after)
	}
	if !kept {
		t.Fatal("the surviving process was the one forgotten")
	}
	// Forgetting everything empties it rather than erroring.
	cache.Forget(map[int]bool{})
	cache.mu.Lock()
	empty := len(cache.entries)
	cache.mu.Unlock()
	if empty != 0 {
		t.Fatalf("the cache holds %d entries after forgetting all, want 0", empty)
	}
}

// Command is the fallback chain a COMMAND column reads, and the order matters: the
// command line says most, the image path says something, and the name is always
// there. An empty column would be the worst answer and is the one this prevents.
func TestDetails_commandFallsBackInOrder(t *testing.T) {
	for _, test := range []struct {
		name    string
		details Details
		want    string
	}{
		{name: "a command line wins", details: Details{CommandLine: "cmd --flag", Path: `C:\p.exe`}, want: "cmd --flag"},
		{name: "the path when there is no command line", details: Details{Path: `C:\p.exe`}, want: `C:\p.exe`},
		{name: "the name when there is neither", details: Details{}, want: "fallback.exe"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.details.Command("fallback.exe"); got != test.want {
				t.Fatalf("Command() = %q, want %q", got, test.want)
			}
		})
	}
	// Even with no name at all the answer is a string rather than a panic, which is
	// what a row for a process that vanished mid-sample looks like.
	if got := (Details{}).Command(""); got != "" {
		t.Fatalf("Command(\"\") = %q", got)
	}
}

// A pid that cannot be opened answers empty details rather than failing, because a
// monitor listing four hundred processes will be refused by most of them -- they
// belong to SYSTEM or to another user -- and one refusal must not end the sample.
func TestReadDetails_refusalIsNotAnError(t *testing.T) {
	for _, pid := range []int{0, -1, 0x7ffffff} {
		details := readDetails(pid)
		if details.CommandLine != "" || details.Path != "" {
			t.Errorf("readDetails(%d) invented %+v", pid, details)
		}
	}
	// And the process this test is in *can* be opened, so the empty answers above
	// are refusals rather than the function never working.
	if own := readDetails(os.Getpid()); own.CommandLine == "" && own.Path == "" {
		t.Fatal("readDetails could not read this process either, so it never works")
	}
}
