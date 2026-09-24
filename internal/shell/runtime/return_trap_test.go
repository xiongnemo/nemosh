package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The RETURN trap, measured against bash 5.3: the trap a call sets fires as it returns and
// stays set; a function does not inherit one unless `set -T`; a sourced file finishing
// fires it; `exit` does not.
func TestReturnTrap_firesAsTheCallThatSetItReturns(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "set in the call", script: "f() { trap 'echo ret-f' RETURN; echo in-f; return 3; }\nf\necho \"st=$?\"\ng() { echo in-g; }\ng\n", want: "in-f\nret-f\nst=3\nin-g\n"},
		{name: "not by a call inside it", script: "f() { trap 'echo ret-f' RETURN; g; echo body; }\ng() { echo in-g; }\nf\n", want: "in-g\nbody\nret-f\n"},
		{name: "inherited under set -T", script: "set -T\ntrap 'echo ret' RETURN\nf() { :; }\nf\n", want: "ret\n"},
		{name: "sees the call's locals and stays set", script: "f() { local x=1; trap 'echo ret x=$x' RETURN; }\nf\ntrap -p RETURN\n", want: "ret x=1\ntrap -- 'echo ret x=$x' RETURN\n"},
		{name: "not after exit", script: "f() { trap 'echo ret' RETURN; exit 4; }\nf\necho no\n", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

func TestReturnTrap_firesAsASourcedFileFinishes(t *testing.T) {
	sourced := filepath.Join(t.TempDir(), "lib.sh")
	if err := os.WriteFile(sourced, []byte("echo in-sourced\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "trap 'echo ret-top' RETURN\nf() { echo in-f; }\nf\n. '" + filepath.ToSlash(sourced) + "'\necho end\n"
	if stdout, _ := runScriptCapturing(script); stdout != "in-f\nin-sourced\nret-top\nend\n" {
		t.Fatalf("stdout = %q", stdout)
	}
}

// `trap -p` prints and `trap -l` lists, as in bash; `trap -p RETURN` armed RETURN with a
// command named -p. DEBUG is refused with its reason rather than called invalid.
func TestTrap_printsListsAndRefusesDebug(t *testing.T) {
	stdout, _ := runScriptCapturing("trap 'echo x' EXIT\ntrap -p EXIT\ntrap -- 'echo y' EXIT\ntrap -p\ntrap -l | head -1\ntrap - EXIT\n")
	if stdout != "trap -- 'echo x' EXIT\ntrap -- 'echo y' EXIT\n 1) SIGHUP\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	status, _, stderr := runSetScript(t, "trap 'echo d' DEBUG\n")
	if status != 1 || !strings.Contains(stderr, "DEBUG: not implemented") {
		t.Fatalf("trap DEBUG = %d %q, want a refusal saying why", status, stderr)
	}
}
