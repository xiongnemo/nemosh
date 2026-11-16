package main

import (
	"bufio"
	"os"
	"strconv"
	"testing"

	"golang.org/x/sys/windows"
)

// The test binary stands in for nemosh: started with STARTBENCH_SESSION set, it commits that many
// bytes, touches none of them, and waits as a session does, so commitOf has a known commit to find.
func TestMain(m *testing.M) {
	if size, set := os.LookupEnv("STARTBENCH_SESSION"); set {
		if bytes, _ := strconv.Atoi(size); bytes > 0 {
			if _, err := windows.VirtualAlloc(0, uintptr(bytes), windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE); err != nil {
				os.Exit(3)
			}
		}
		os.Stdout.WriteString("ready\n")
		bufio.NewReader(os.Stdin).ReadString('\n')
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// commitOf finds what a waiting session has committed whether it touched it or not: 64 MiB
// committed and never written reads as about that much more than a session that committed
// nothing. About: the rest of a session's commit is not the same from one start to the next, and
// under the gate one came out 72 KiB short of 64 MiB.
func TestCommitOf_findsWhatASessionCommittedUntouched(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("STARTBENCH_SESSION", "0")
	small, err := commitOf(self)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("STARTBENCH_SESSION", strconv.Itoa(64<<20))
	large, err := commitOf(self)
	if err != nil {
		t.Fatal(err)
	}
	if large < small+60<<20 || large > small+80<<20 {
		t.Errorf("a session that committed 64 MiB more read as %d bytes, against %d", large, small)
	}
}
