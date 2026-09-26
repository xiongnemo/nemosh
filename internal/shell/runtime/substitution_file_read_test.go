package runtime_test

import "testing"

// `$(< file)` is the file's contents, trailing newlines trimmed as any substitution's are:
// bash's shorthand for `$(cat file)`, and a common one. It was the empty output of a command
// with no words, which is what busybox-w32 gives and what POSIX leaves it; bash decides an
// extension busybox lacks. A file that cannot be opened is reported, with status 1, as both
// references report it. Another redirection alongside makes it an ordinary command again, in
// bash too.
func TestRuntime_substitutionOfARedirectionReadsTheFile(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`f=$(mktemp); printf 'a\nb\n\n' > "$f"; x=$(< "$f"); rm -f "$f"; printf '[%s]\n' "$x"`, "[a\nb]\n"},
		{`f=$(mktemp); printf 'hi\n' > "$f"; x=$(<"$f"); rm -f "$f"; echo "[$x]"`, "[hi]\n"},
		{`f=$(mktemp); printf 'hi\n' > "$f"; echo "$(< $f) there"; rm -f "$f"`, "hi there\n"},
		{`x=$(< /no/such/file 2>/dev/null); echo "status=$? [$x]"`, "status=1 []\n"},
		{`f=$(mktemp); printf 'hi\n' > "$f"; x=$(< "$f" 2>/dev/null); rm -f "$f"; echo "[$x]"`, "[]\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
