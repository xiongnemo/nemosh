package runtime_test

import (
	"strings"
	"testing"
)

// A for, select or case header may have its `in` on a later line, as POSIX's grammar and bash's
// have it and busybox-w32 reads it. The loop ran over "$@" with its list dropped, and the case
// said it had no `in`.
func TestRuntime_inMayBeginTheNextLine(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{"for x\nin a b; do echo $x; done", "a\nb\n"},
		{"set -- p q\nfor x\n\n# the list\nin a b\ndo echo $x\ndone", "a\nb\n"},
		{"if true; then for x\nin a b; do echo $x; done; fi", "a\nb\n"},
		{"echo pre | for x\nin a b; do echo $x; done", "a\nb\n"},
		{"for x\nin\ndo echo none; done; echo st=$?", "st=0\n"},
		{"case a\nin a) echo matched;; esac", "matched\n"},
		{"case a # c\n\nin\na|b) echo ab;;\nesac", "ab\n"},
		{"case $(echo a)\nin a) echo sub;; esac", "sub\n"},
		{"f() {\n  case $1\n  in\n    x) echo x;;\n    *) echo other;;\n  esac\n}\nf x; f y", "x\nother\n"},
		{"x=$(case a\nin a) echo sub;; esac); echo $x", "sub\n"},
	} {
		if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
			t.Errorf("%d: %q: got %q/%d, want %q/0, as both references answer", index, test.script, stdout, status, test.want)
		}
	}
}

// Nothing else may stand between a for or select header and its do: both references refuse
// `for x`, `echo oops`, `do`. The line was dropped and the loop ran as though it were not there.
func TestRuntime_aLineBeforeDoIsASyntaxError(t *testing.T) {
	for _, script := range []string{
		"for x\necho oops\ndo echo $x; done\necho after",
		"for x in a b\necho oops\ndo echo $x; done\necho after",
		"select x in a\necho oops\ndo break; done\necho after",
		"for i; in one two three; do echo $i; done\necho after",
	} {
		stdout, status := runScriptCapturing(script + "\n")
		if status != 2 || strings.Contains(stdout, "after") {
			t.Errorf("%q: got %q/%d, want a syntax error and nothing run", script, stdout, status)
		}
	}
}
