package runtime_test

import (
	"fmt"
	"path/filepath"
	goruntime "runtime"
	"testing"
)

// On Windows, PATH, CDPATH and MANPATH take a `:` between their directories as well as `;`,
// as busybox-w32 has them: an assignment rewrites each `:` that is not a drive letter's into
// `;`. `PATH=bin:$PATH`, how a script written for Unix puts a directory first, was one
// directory here, named `bin:C:\...`, and nothing was found in it. The answers are
// busybox-w32's, down to a one-letter directory at the start of a list reading as a drive.
// The program is looked up and not run: run, it would be this test binary.
func TestRuntime_windowsPathListTakesColons(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("busybox-w32's rule, which is for Windows alone")
	}
	script := fmt.Sprintf("cd '%s'\n", filepath.ToSlash(t.TempDir())) +
		"mkdir bin; printf MZ > bin/prog.exe\n" +
		"found() { case $1 in *bin/prog.exe) echo found;; *) echo \"missing: $1\";; esac; }\n" +
		"( PATH=\"bin:$PATH\"; found \"$(command -v prog)\" )\n" +
		"found \"$(PATH=\"bin:$PATH\" command -v prog)\"\n" +
		"( export PATH=\"bin:$PATH\"; echo \"${PATH%%;*}\" )\n" +
		"x='bin:C:\\x;D:/y:/c/z'; ( PATH=$x; echo \"$PATH\" )\n" +
		"CDPATH=.:/tmp; OTHER=ab:cd; echo \"$CDPATH $OTHER\"\n" +
		"( PATH='x:C:y:D:'; echo \"$PATH\" )\n"
	want := "found\nfound\nbin\nbin;C:\\x;D:/y;/c/z\n.;/tmp ab:cd\nx:C;y:D;\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q/%d, want %q/0, as busybox-w32 answers", stdout, status, want)
	}
}
