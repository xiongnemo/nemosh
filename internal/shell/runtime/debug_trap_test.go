package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// debugLog is the Oils suite's reporter: the trap says the line of each command it runs for.
const debugLog = "debuglog() { echo \"  [$*]\"; }\ntrap 'debuglog $LINENO' DEBUG\n"

// The DEBUG trap runs before each simple command, before each turn of a for or select loop,
// before each part of an arithmetic for and before a case, with $BASH_COMMAND the command about
// to run. It leaves $? alone, and ends the shell only by `exit` or under `set -e`. A function, a
// sourced file and a subshell do not inherit it unless `set -T`; a pipeline's simple commands
// have theirs run in the shell before the pipeline starts. Every answer is bash 5.3's, measured;
// busybox-w32 has no DEBUG trap, and here it was refused as not implemented.
func TestDebugTrap_runsBeforeEachCommand(t *testing.T) {
	for index, test := range []struct {
		script, want string
		status       int
	}{
		{"debuglog() { echo \"  [$*]\"; return 42; }\ntrap 'debuglog $LINENO' DEBUG\necho status=$?\necho A\necho status=$?\n", "  [3]\nstatus=0\n  [4]\nA\n  [5]\nstatus=0\n", 0},
		{"set -e\ndebuglog() { echo \"  [$*]\"; return 42; }\ntrap 'debuglog $LINENO' DEBUG\necho A\necho B\n", "  [4]\n", 42},
		{"trap 'echo \"  [$LINENO]\"; exit 42' DEBUG\necho A\necho B\n", "  [2]\n", 42},
		{debugLog + "echo a\necho b; echo c\necho d && echo e\necho f || echo g\n(( h = 42 ))\n[[ j == j ]]\nvar=value\nreadonly r=value\n", "  [3]\na\n  [4]\nb\n  [4]\nc\n  [5]\nd\n  [5]\ne\n  [6]\nf\n  [7]\n  [8]\n  [9]\n  [10]\n", 0},
		{debugLog + "while true; do\n  echo hello\n  break\ndone\n", "  [3]\n  [4]\nhello\n  [5]\n", 0},
		{debugLog + "echo \"r=$(echo sub; echo two)\"\n( echo subshell\n  echo two\n)\necho done\n", "  [3]\nr=sub\ntwo\nsubshell\ntwo\n  [7]\ndone\n", 0},
		{debugLog + "{ echo pipe1; echo pipe2; } | cat\necho ok\n", "  [3]\npipe1\npipe2\n  [4]\nok\n", 0},
		{debugLog + "echo pipeline | cat\necho ok\n", "  [3]\n  [3]\npipeline\n  [4]\nok\n", 0},
		{debugLog + "f() {\n  local mylocal=1\n  for i in \"$@\"; do\n    echo i=$i\n  done\n}\nf A B\necho next\n", "  [9]\ni=A\ni=B\n  [10]\nnext\n", 0},
		{debugLog + "name=foo.py\ncase $name in\n  *.py)\n    echo python\n    ;;\n  *.sh)\n    echo shell\n    ;;\nesac\necho ok\n", "  [3]\n  [4]\n  [6]\npython\n  [12]\nok\n", 0},
		{debugLog + "for x in 1 2; do\n  echo x=$x\ndone\necho ok\n", "  [3]\n  [4]\nx=1\n  [3]\n  [4]\nx=2\n  [6]\nok\n", 0},
		{debugLog + "set -- p q\nfor x; do\n  echo x=$x\ndone\n", "  [3]\n  [4]\n  [5]\nx=p\n  [4]\n  [5]\nx=q\n", 0},
		{debugLog + "for (( i = 3; i < 5; ++i )); do\n  echo i=$i\ndone\necho ok\n", "  [3]\n  [3]\n  [4]\ni=3\n  [3]\n  [3]\n  [4]\ni=4\n  [3]\n  [3]\n  [6]\nok\n", 0},
		{debugLog + "if test x = x; then\n  echo if\nfi\nwhile test x != x; do\n  echo while\ndone\n", "  [3]\n  [4]\nif\n  [6]\n", 0},
		{"trap 'echo dbg $LINENO' DEBUG\nfalse | false | false\nfalse || false || false\n! true\ntrap - DEBUG\necho ok\n", "dbg 2\ndbg 2\ndbg 2\ndbg 3\ndbg 3\ndbg 3\ndbg 4\ndbg 5\nok\n", 0},
		{"trap 'false; echo $LINENO err' ERR\ntrap 'false; echo $LINENO debug' DEBUG\nfalse\necho after=$?\n", "3 err\n3 debug\n3 debug\n3 debug\n3 err\n4 err\n4 debug\nafter=1\n", 0},
		{"trap 'echo \"[$BASH_COMMAND]\"' DEBUG\necho a\nx=1\n", "[echo a]\na\n[x=1]\n", 0},
		{"trap 'echo d' DEBUG\ntrap -p DEBUG\ntrap - DEBUG\ntrap -p DEBUG\necho end\n", "d\ntrap -- 'echo d' DEBUG\nd\nend\n", 0},
		{"trap 'echo \"[$?]\"' DEBUG\nfalse\ntrue\necho end\n", "[0]\n[1]\n[0]\nend\n", 0},
		{debugLog + "eval 'echo one; echo two'\n", "  [3]\n  [3]\none\n  [3]\ntwo\n", 0},
		{"trap 'echo dbg' DEBUG\ntrap 'echo bye' EXIT\necho last\n", "dbg\ndbg\nlast\ndbg\nbye\n", 0},
		{"trap 'echo \"[$LINENO]\"' DEBUG\ntrue &\ntrue | true &\ntrue && true &\n{ true; } &\nwait\n", "[2]\n[3]\n[3]\n[6]\n", 0},
		{"set -T\ntrap 'echo \"[$BASH_COMMAND]\"' DEBUG\n( echo sub )\nx=$(echo hi)\necho \"$x\"\n", "[echo sub]\nsub\n[x=$(echo hi)]\n[echo \"$x\"]\n[echo hi]\nhi\n", 0},
		{"set -T\ntrap 'v=$(echo t)' DEBUG\necho a\necho \"v=$v\"\n", "a\nv=t\n", 0},
		{"trap 'echo \"[$BASH_COMMAND]\"' DEBUG\nselect z in c; do echo \"z=$z\"; break; done <<< $'\\n1'\n", "[select z in c]\n[echo \"z=$z\"]\nz=c\n[break]\n", 0},
		{"trap 'echo \"[$BASH_COMMAND]\"' DEBUG\nfor x in 1 \"2 3\"; do :; done\nfor ((i = 0; i < 1; i++)); do :; done\nfor ((;;)); do break; done\ncase foo in f*) : ;; esac\nset -- a\nfor w; do :; done\n", "[for x in 1 \"2 3\"]\n[:]\n[for x in 1 \"2 3\"]\n[:]\n[((i = 0))]\n[((i < 1))]\n[:]\n[((i++))]\n[((i < 1))]\n[((1))]\n[((1))]\n[break]\n[case foo in ]\n[:]\n[set -- a]\n[for w in \"$@\"]\n[:]\n", 0},
		{"trap 'echo d' DEBUG\nf() { trap -p DEBUG; echo in-f; }\nf\ng() { trap 'echo G' DEBUG; }\ng\necho after\n", "d\nin-f\nd\nG\nafter\n", 0},
		{"trap 'echo \"[$LINENO]\"' DEBUG\necho a > /dev/null\n{ echo g; } > /dev/null\necho end\n", "[2]\n[4]\nend\n", 0},
		{"trap 'echo \"[$BASH_COMMAND]\"' DEBUG\nf() { echo f-out; }\nf | cat\n", "[f]\n[cat]\nf-out\n", 0},
		{"trap '' DEBUG\necho a\ntrap -p DEBUG\n", "a\ntrap -- '' DEBUG\n", 0},
		{"set -e\ntrap 'false' DEBUG\nif true; then echo in; fi\necho after\n", "", 1},
		{"trap 'echo \"D[$BASH_COMMAND]\"' DEBUG\ntrap 'echo \"E[$BASH_COMMAND]\"' ERR\nfalse x\n", "D[trap 'echo \"E[$BASH_COMMAND]\"' ERR]\nD[false x]\nD[false x]\nE[false x]\n", 1},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != test.status {
			t.Errorf("%d: %q: got %q/%d, want %q/%d, as bash answers", index, test.script, stdout, status, test.want, test.status)
		}
	}
}

// A sourced file does not inherit the trap, as a function does not: `trap -p` there shows none.
// One it sets is its own and stays set after it, as in bash.
func TestDebugTrap_sourcedFileKeepsOnlyItsOwn(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{"lib.sh": "echo in-lib\ntrap -p DEBUG\n", "own.sh": "trap 'echo own' DEBUG\necho in-own\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct{ file, want string }{
		{"lib.sh", "d\nin-lib\nd\nback\n"},
		{"own.sh", "d\nown\nin-own\nown\nback\n"},
	} {
		path := strings.ReplaceAll(filepath.ToSlash(filepath.Join(dir, test.file)), "'", `'\''`)
		script := "trap 'echo d' DEBUG\n. '" + path + "'\necho back\n"
		if stdout, status := runScriptCapturing(script); stdout != test.want || status != 0 {
			t.Errorf("%s: got %q/%d, want %q/0, as bash answers", test.file, stdout, status, test.want)
		}
	}
}
