package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
)

// `!(` where a command begins negates a subshell, as busybox-w32 and bash both run it. The word
// was taken for an extended pattern even there, so it was matched against the directory and
// the first file it matched run: here `t.sh`, not found. Elsewhere -- a case pattern, one
// after `;;`, the right side of `==` -- it is the pattern it was.
func TestRuntime_bangParenthesisNegatesASubshell(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "t.sh"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	negations := "if !(false && false); then echo one; fi; echo two\n" +
		"!(false) && echo yes\n" +
		"x=false; !($x) && echo yes2\n" +
		"true && !(false) && echo yes3\n" +
		"f() { !(true) || echo in-f; }; f\n"
	patterns := "case b in !(a)) echo not-a;; esac\n" +
		"case a in a) ;; !(a)) echo never;; esac\n" +
		"[[ b == !(a) ]] && echo pattern\n"
	for _, test := range []struct{ script, want string }{
		{negations, "one\ntwo\nyes\nyes2\nyes3\nin-f\n"},
		{patterns, "not-a\npattern\n"},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
			t.Errorf("%q: got %q/%d, want %q/0", test.script, stdout, status, test.want)
		}
	}
}
