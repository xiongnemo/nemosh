package applets

import (
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// `-H` and the `H` key, which took an option and did nothing until now.
//
// The test that matters most is the first one, and it is embarrassingly simple: the row count has to
// change. That is the whole of what was wrong -- `top -b -n 1 -H | wc -l` and the same without `-H`
// both answered 561 -- and no test noticed, because every test asked whether the rows were correct
// and none asked whether the option did anything at all.

func threadedProcess(pid int, name string, threads ...proc.Thread) proc.Process {
	process := testProcess(pid, 0, name, 1<<20)
	process.Threads = len(threads)
	process.ThreadDetail = threads
	return process
}

func testThread(id int, used time.Duration) proc.Thread {
	return proc.Thread{
		ID: id, Created: modelEpoch, Kernel: used, User: 0,
		Priority: 8, State: proc.StateRunning,
	}
}

func TestTopModel_dashHAddsARowPerThread(t *testing.T) {
	snapshot, rates := testSnapshot(
		threadedProcess(100, "two.exe", testThread(1001, time.Second), testThread(1002, time.Second)),
		threadedProcess(200, "one.exe", testThread(2001, time.Second)),
	)
	model := newTopModel(mustColumns(t))

	// When -- off
	without := model.rows(snapshot, rates, proc.NewDetailCache())

	// And on
	model.applyKey("H")
	with := model.rows(snapshot, rates, proc.NewDetailCache())

	// Then -- two processes, then two processes and their three threads
	if len(without) != 2 {
		t.Fatalf("without -H there are %d rows, want 2", len(without))
	}
	if len(with) != 5 {
		t.Fatalf("with -H there are %d rows, want 5 -- two processes and three threads", len(with))
	}
	// Threads sit under their own process rather than being sorted in among them.
	if with[0].Process.PID != 100 || with[0].Thread != nil {
		t.Fatalf("row 0 is %v, want the process 100", rowNames(with[:1]))
	}
	for _, index := range []int{1, 2} {
		if with[index].Thread == nil || with[index].Process.PID != 100 {
			t.Fatalf("row %d is not a thread of process 100", index)
		}
	}
	if with[3].Thread != nil || with[3].Process.PID != 200 {
		t.Fatal("the second process does not follow its predecessor's threads")
	}
}

// A thread row answers for the thread where a thread has its own figure, and for its process where
// the figure belongs to the process.
func TestTopRow_threadRowsAnswerForTheThread(t *testing.T) {
	thread := proc.Thread{ID: 4242, Created: modelEpoch, Kernel: 90 * time.Second, Priority: 13,
		State: proc.StateWaiting}
	process := threadedProcess(100, "host.exe", thread)
	row := topRow{Process: process, Rate: proc.ProcessRate{CPU: 0.5},
		Thread: &thread, ThreadRate: proc.ThreadRate{CPU: 0.25}}

	// Then -- the thread's own id, priority, state, CPU and time
	if row.id() != 4242 {
		t.Fatalf("id = %d, want the thread id", row.id())
	}
	if row.priority() != 13 || row.state() != proc.StateWaiting {
		t.Fatalf("priority = %d state = %c, want the thread's", row.priority(), row.state())
	}
	if row.cpu() != 0.25 || row.cpuTime() != 90*time.Second {
		t.Fatalf("cpu = %v time = %v, want the thread's", row.cpu(), row.cpuTime())
	}

	// And the process's name, because a thread has none to show.
	if row.Process.Name != "host.exe" {
		t.Fatalf("name = %q, want the owning process's", row.Process.Name)
	}

	// And nothing at all for the figures a thread does not have. Repeating them would read as
	// each of a process's thirty threads owning its whole working set.
	for _, key := range []string{"mem", "rss", "private", "commit", "thr", "handles",
		"read", "write", "other", "iops", "readops", "writeops"} {
		column, ok := columnByKey(key)
		if !ok {
			t.Fatalf("no %s column", key)
		}
		if got := topCellText(column, row); got != "" {
			t.Fatalf("a thread row shows %q for %s, which belongs to the process",
				got, key)
		}
		// The process row does show it, or the column would be pointless.
		if topCellText(column, topRow{Process: process}) == "" && key != "read" && key != "write" &&
			key != "other" && key != "iops" && key != "readops" && key != "writeops" {
			t.Fatalf("the process row shows nothing for %s either", key)
		}
	}
}

// The plain form and the drawn form blank the same cells, because they go through one function.
func TestTopRowLine_threadRowsAreBlankWhereTheProcessRowIsNot(t *testing.T) {
	thread := testThread(4242, time.Minute)
	process := threadedProcess(100, "host.exe", thread)
	// Without the command, which is the one column whose width is meant to vary: it takes the
	// rest of the line, and a thread's is indented under its process. Comparing whole lines
	// measured that indent and called it a misalignment.
	columns, _ := resolveColumns([]string{"pid", "rss", "thr"})

	// When
	processLine := topRowLine(columns, topRow{Process: process})
	threadLine := topRowLine(columns, topRow{Process: process, Thread: &thread, Depth: 1})

	// And the command is indented, so a thread reads as belonging to the row above it.
	withCommand, _ := resolveColumns([]string{"pid", "command"})
	indented := topRowLine(withCommand, topRow{Process: process, Thread: &thread, Depth: 1})
	if !strings.Contains(indented, "  host.exe") {
		t.Fatalf("a thread's command is not indented under its process: %q", indented)
	}

	// Then -- the fixed columns are the same width on both, so the table lines up
	if len(processLine) != len(threadLine) {
		t.Fatalf("process line is %d wide and its thread's is %d:\n%q\n%q",
			len(processLine), len(threadLine), processLine, threadLine)
	}
	if !strings.Contains(processLine, "1.0M") {
		t.Fatalf("the process line has no working set: %q", processLine)
	}
	if strings.Contains(threadLine, "1.0M") {
		t.Fatalf("the thread line repeats its process's working set: %q", threadLine)
	}
	if !strings.Contains(threadLine, "4242") {
		t.Fatalf("the thread line does not show its thread id: %q", threadLine)
	}
}
