package main

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// The commit is held to a ceiling: a session 64 MiB over what the ceiling allows fails, with a
// line that says so, and one under it passes. The ceiling is set from what the test binary
// itself commits, which is not nemosh's and differs from one build of it to the next.
func TestReportCommit_failsPastTheCeiling(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("STARTBENCH_SESSION", "0")
	floor, err := commitOf(self)
	if err != nil {
		t.Fatal(err)
	}
	ceiling := int(mebibytes(floor)) + 32
	var out strings.Builder
	if !reportCommit(options{head: self, commitCeiling: ceiling}, &out) {
		t.Errorf("a session under the ceiling failed: %s", out.String())
	}
	t.Setenv("STARTBENCH_SESSION", strconv.Itoa(64<<20))
	out.Reset()
	if reportCommit(options{head: self, commitCeiling: ceiling}, &out) || !strings.Contains(out.String(), "past the ceiling") {
		t.Errorf("a session 64 MiB more passed: %s", out.String())
	}
}
