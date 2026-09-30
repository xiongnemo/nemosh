package runtime_test

import "testing"

// A # after a backslash-newline follows the last character of the line before, and belongs to
// its word when that character does, as busybox and bash read it: `${x\` then `#a}` is
// ${x#a}. It was taken for a comment, the } went with it, and the script was refused as missing
// one. After a blank, or a line that ends in an operator, it begins a comment as before.
// busybox's ash_test bkslash_newline4.
func TestRuntime_aHashAfterABackslashNewlineContinuesItsWord(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"set -- 1 22 333\necho 3:$\\\n{\\\n#\\\n3\\\n}\n", "3:3\n"},
		{"set -- 1 22 333\necho 22:$\\\n{\\\n2\\\n}\n", "22:22\n"},
		{"x=abc\necho ${x\\\n#a}\n", "bc\n"},
		{"echo a\\\n#b\n", "a#b\n"},
		{"echo a \\\n# comment\necho next\n", "a\nnext\n"},
		{"true &&\n# comment\necho after\n", "after\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
