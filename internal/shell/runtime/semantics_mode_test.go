package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Where bash's semantics and busybox's part, bash mode takes bash's: a file `. FILE ARGS` sources
// keeps the positional parameters it set; `[[ A -eq B ]]` evaluates its operands as arithmetic;
// trap lists a signal as SIGINT; `unset f`, with no variable f, unsets the function; the
// assignment before a command whose redirection fails is made; and a function's name may hold a
// slash. Measured 2026-11-21, a script per case, with Git for Windows' bash 5.3 and busybox-w32;
// nemosh and sh keep busybox's.
func TestSemantics_bashModeTakesBashs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.sh"), []byte("set -- from-lib\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib := filepath.ToSlash(filepath.Join(dir, "lib.sh"))
	for _, test := range []struct {
		code, bash, busybox string
	}{
		{code: "set -- outer; . '" + lib + "' one two; echo \"after: $@\"", bash: "after: from-lib\n", busybox: "after: outer\n"},
		{code: "x=2; [[ x+1 -eq 3 ]] && echo arith; [[ \"1+1\" -eq 2 ]] && echo quoted", bash: "arith\nquoted\n", busybox: ""},
		{code: "trap 'echo x' INT; trap", bash: "trap -- 'echo x' SIGINT\n", busybox: "trap -- 'echo x' INT\n"},
		{code: "f() { echo fn; }; unset f; f 2>/dev/null; echo st=$?", bash: "st=127\n", busybox: "fn\nst=0\n"},
		{code: "x=1 > /no/such/dir/f; echo \"x=${x-unset}\"", bash: "x=1\n", busybox: "x=unset\n"},
		{code: "a/b() { echo slash-fn; }; a/b 2>/dev/null; echo st=$?", bash: "slash-fn\nst=0\n", busybox: "st=127\n"},
	} {
		for _, bash := range []bool{false, true} {
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
			want := test.busybox
			if bash {
				rt.EnterBashMode()
				want = test.bash
			}
			rt.RunScript(context.Background(), test.code+"\n")
			if got := stdout.String(); got != want {
				t.Errorf("%q, bash mode %v: stdout %q, want %q; stderr %q", test.code, bash, got, want, stderr.String())
			}
		}
	}
	if strings.Contains(lib, "'") {
		t.Fatalf("a temporary directory with a quote in its name: %q", lib)
	}
}
