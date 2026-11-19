package proc

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// Watch is tail --pid's: whether a process has ended, asked as often as tail looks. A pid that
// names no process has ended already, which is the answer rather than an error.
func TestWatch_seesAProcessEnd(t *testing.T) {
	self, err := Watch(os.Getpid())
	if err != nil {
		t.Fatalf("Watch(self): %v", err)
	}
	defer self.Close()
	if self.Ended() {
		t.Error("this process has ended, Watch says")
	}

	// OpenProcess ignores a pid's low two bits, so one above this process's opens this process.
	// No process has that pid, for a Windows pid is a multiple of four.
	beside, err := Watch(os.Getpid() + 1)
	if err != nil {
		t.Fatalf("Watch(self+1): %v", err)
	}
	defer beside.Close()
	if !beside.Ended() {
		t.Errorf("pid %d, which no process can have, is running, Watch says", os.Getpid()+1)
	}

	for _, pid := range []int{0, -4} {
		if watcher, err := Watch(pid); err == nil {
			watcher.Close()
			t.Errorf("Watch(%d) succeeded", pid)
		}
	}
}

func TestWatch_seesAChildEnd(t *testing.T) {
	child := exec.Command(os.Args[0], "-test.run=^TestWatchHelper$")
	child.Env = append(os.Environ(), "NEMOSH_WATCH_HELPER=1")
	if err := child.Start(); err != nil {
		t.Fatalf("starting the helper: %v", err)
	}
	defer child.Process.Kill()
	watcher, err := Watch(child.Process.Pid)
	if err != nil {
		t.Fatalf("Watch(child): %v", err)
	}
	defer watcher.Close()
	if watcher.Ended() {
		t.Fatal("the child has ended before it was stopped, Watch says")
	}
	child.Process.Kill()
	child.Wait()
	deadline := time.Now().Add(10 * time.Second)
	for !watcher.Ended() {
		if time.Now().After(deadline) {
			t.Fatal("the child was stopped and Watch never saw it end")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestWatchHelper is the child TestWatch_seesAChildEnd watches: it waits to be stopped.
func TestWatchHelper(t *testing.T) {
	if os.Getenv("NEMOSH_WATCH_HELPER") == "" {
		t.Skip("the child of TestWatch_seesAChildEnd")
	}
	time.Sleep(time.Minute)
}
