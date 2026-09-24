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

// FUNCNAME as an array, BASH_SOURCE, BASH_LINENO and caller, across a sourced file and two
// calls. The transcript is bash 5.3's on the same two files, measured; all four were unset
// or not found.
func TestCallStack_reportsFunctionsFilesAndLines(t *testing.T) {
	lib := filepath.ToSlash(filepath.Join(t.TempDir(), "lib.sh"))
	body := "libf() {\n  echo \"[${FUNCNAME[*]}] [${BASH_SOURCE[*]}] [${BASH_LINENO[*]}]\"\n  caller 0\n  caller 1\n  caller 2 || echo past\n}\n"
	if err := os.WriteFile(lib, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	script := ". '" + lib + "'\ng() {\n  libf\n}\n\ng\necho \"[${BASH_SOURCE[*]}] [${#FUNCNAME[@]}] [$BASH_SOURCE]\"\ncaller\n"
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	rt.SetScriptFile("main.sh")
	status := rt.RunScript(context.Background(), script)
	rt.CloseBatch(status)

	want := "[libf g main] [" + lib + " main.sh main.sh] [3 6 0]\n3 g main.sh\n6 main main.sh\npast\n[main.sh] [0] [main.sh]\n0 NULL\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

// declare, typeset and shopt run as builtins and were not listed as ones, so `type` said
// not found and `command -v` said nothing; caller is new.
func TestCallStack_builtinsAreListed(t *testing.T) {
	if stdout, _ := runScriptCapturing("type -t declare typeset shopt caller\n"); stdout != "builtin\nbuiltin\nbuiltin\nbuiltin\n" {
		t.Fatalf("type -t = %q", stdout)
	}
}
