package runtime_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// cd prints where it went when a CDPATH entry that is not empty supplied the directory, as
// POSIX has it and both references do, so `cd sub` from anywhere says which sub it chose. It
// printed nothing. One found in the current directory, by an empty entry or by none, is not
// printed.
func TestRuntime_cdPrintsADirectoryCDPATHFound(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	script := fmt.Sprintf("cd '%s'\nmkdir -p base/sub other/sub2 here\n", dir) +
		"CDPATH=base; cd sub; echo \"st=$? $PWD\"; cd ../..\n" +
		"CDPATH=base:other; cd sub2 >/dev/null; echo st=$?; cd ../..\n" +
		"CDPATH=:base; cd here; echo st=$?\n"
	stdout, status := runScriptCapturing(script)
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if status != 0 || len(lines) != 4 {
		t.Fatalf("got %q/%d, want four lines and status 0", stdout, status)
	}
	if !strings.HasSuffix(lines[0], "/base/sub") || lines[1] != "st=0 "+lines[0] {
		t.Errorf("got %q, want the directory CDPATH found, and then $PWD being it", lines[:2])
	}
	if lines[2] != "st=0" || lines[3] != "st=0" {
		t.Errorf("got %q, want the redirected print gone and nothing for the empty entry", lines[2:])
	}
}
