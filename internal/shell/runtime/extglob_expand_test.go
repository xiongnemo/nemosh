package runtime_test

import (
	"fmt"
	"path/filepath"
	"testing"
)

// An extended pattern globs as the other pattern characters do, and what is inside its group
// is expanded and quoted as any word is. A pathname was a pattern only with `*`, `?` or `[` in
// it, so `rm !(keep)` and `echo @(foo|bar).py` stayed as written; and a group went into the
// word as its literal characters, so `?($ext|h)` looked for a file with a `$` in its name and
// `@(a|'*')` kept its quotes. The answers are bash 5.3's with extglob on; busybox has none.
func TestRuntime_extendedGlobExpands(t *testing.T) {
	tests := []struct {
		setup, script, want string
	}{
		{"touch foo.py bar.py baz.txt", "echo @(foo|bar).py; echo !(foo).py", "bar.py foo.py\nbar.py\n"},
		{"touch foo foo.cc foo.h foo.hh", "ext=.cc; echo foo?($ext|.h); echo foo.@(c$(echo c)|h)", "foo foo.cc foo.h\nfoo.cc foo.h\n"},
		{"touch foo.py bar.py", "x='fo*'; echo @(\"$x\"|bar).py; p='@(foo|bar).py'; echo $p; echo \"@(foo|bar)\".py", "bar.py\nbar.py foo.py\n@(foo|bar).py\n"},
		{"mkdir 2; touch 2/aa 2/ab 2/ac 2/ba 2/bb 2/bc 2/ca 2/cb 2/cc", "echo 2/!(b)@(b|c); echo 2/a@(!(c|a))", "2/ab 2/ac 2/cb 2/cc\n2/ab\n"},
		{"touch 'a b' c", "x='a b'; printf '<%s>' @($x|c) @(\"$x\"|c); echo", "<@(a><b|c)><a b><c>\n"},
		{"", "set -f; echo @(a|'*'|b); y=b; x=@(a|$y); echo \"$x\"; echo @(nope|none)", "@(a|*|b)\n@(a|b)\n@(nope|none)\n"},
		{"", "ext=cc; [[ foo.cc == foo.?($ext|h) ]] && echo m; case foo.cc in foo.?($ext|h)) echo m;; esac", "m\nm\n"},
		{"touch foo.py bar.py spam.py", "echo ${undef:-@(foo|bar).py}; x=@(foo|bar).py; echo \"$x\"", "bar.py foo.py\n@(foo|bar).py\n"},
		{"", "[[ '' == @() ]] && echo 1; [[ '' == @(a||b) ]] && echo 2; [[ X == @() ]] || echo 3; [[ '|' == @(||) ]] || echo 4", "1\n2\n3\n4\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			script := fmt.Sprintf("shopt -s extglob\ncd '%s'; %s\n%s\n", filepath.ToSlash(t.TempDir()), test.setup, test.script)
			if stdout, status := runScriptCapturing(script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
