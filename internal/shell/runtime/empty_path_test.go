package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// An empty word names no file to the shell either: `[ -e "" ]` is false, a redirection to it
// fails, `""` is not found, and `rm -rf ""` removes nothing. The path model joined it to the
// working directory, so each of those used that. cd takes an empty directory as the current one,
// `cd ""` and an empty HOME or OLDPWD, as busybox's cdcmd does; pushd refuses it, as bash does.
func TestRuntime_anEmptyWordNamesNoFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	script := "cd '" + filepath.ToSlash(dir) + `'
here=$PWD
[ -e "" ]; echo "e=$?"
[ -d "" ]; echo "d=$?"
test -r ""; echo "r=$?"
( exec 3< "" ) 2>/dev/null; echo "exec=$?"
{ echo hi > ""; } 2>/dev/null; echo "redirect=$?"
x=$(< "") 2>/dev/null; echo "read=$? [$x]"
"" 2>/dev/null; echo "command=$?"
rm -rf ""; echo "rm=$?"
cd ""; echo "cd=$?"
HOME= cd; echo "home=$?"
OLDPWD= cd - > /dev/null; echo "oldpwd=$?"
[ "$PWD" = "$here" ]; echo "stayed=$?"
pushd "" 2>&1; echo "pushd=$?"
`
	rt.RunScript(context.Background(), script)
	want := "e=1\nd=1\nr=1\nexec=1\nredirect=1\nread=1 []\ncommand=127\nrm=0\ncd=0\nhome=0\noldpwd=0\nstayed=0\npushd: null directory\npushd=1\n"
	if got := stdout.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s\nstderr: %s", got, want, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "keep")); err != nil {
		t.Errorf("rm -rf \"\" removed the working directory's file: %v", err)
	}
}
