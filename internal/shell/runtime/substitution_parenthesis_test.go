package runtime_test

import "testing"

// A `)` inside a `$(...)` closes it only when it is the substitution's own, and not a
// subshell's, a case pattern's, a function's or a `<(`'s, as busybox-w32 and bash read it. The
// scan that finds where a logical line ends took every `)` for the substitution's, so inside a
// brace group the real one was `unexpected ), expected }`, and a substitution going on past a
// line with a subshell or a pattern on it was cut off there.
func TestRuntime_substitutionEndsAtItsOwnParenthesis(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{"{ x=$( (echo a) ); echo \"$x\"; }\n", "a\n"},
		{"{ x=$(case a in a) echo hit;; esac); echo \"$x\"; }\n", "hit\n"},
		{"{ x=$(echo \"(\" ')'); echo \"$x\"; }\n", "( )\n"},
		{"x=$( (echo b)\n)\necho \"[$x]\"\n", "[b]\n"},
		{"x=$(case x in\nx) echo hit;;\nesac)\necho \"[$x]\"\n", "[hit]\n"},
		{"x=$(case x in (x) echo paren;; esac\n)\necho \"[$x]\"\n", "[paren]\n"},
		{"x=$(f() { echo fn; }\nf)\necho \"[$x]\"\n", "[fn]\n"},
		{"x=$(f() (echo fs)\nf)\necho \"[$x]\"\n", "[fs]\n"},
		{"x=$(cat <(echo p)\n)\necho \"[$x]\"\n", "[p]\n"},
		{"x=$(\n  (echo a) |\n    ((echo b\n      echo c) | cat)\n)\necho \"[$x]\"\n", "[b\nc]\n"},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
			t.Errorf("%d: %q: got %q/%d, want %q/0, as busybox-w32 and bash answer", index, test.script, stdout, status, test.want)
		}
	}
}
