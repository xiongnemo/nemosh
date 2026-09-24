package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
)

// A duplication's source is expanded, `{name}` picks a descriptor from 10 up, and `>&word`
// with a word that is not a descriptor is a file for both streams. Each answer is bash
// 5.3's, measured; busybox agrees on the first and the last, and has no `{name}`.
func TestRedirect_descriptorsFromWordsAndNames(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "an expanded source",
			script: "exec 3>'" + dir + "/three'\nfd=3\necho hi >&$fd\necho two >&\"$fd\"\nexec 3>&-\ncat '" + dir + "/three'\n",
			want:   "hi\ntwo\n",
		},
		{
			name:   "a named descriptor opens, writes and closes",
			script: "exec {fd}>'" + dir + "/named'\necho \"fd=$fd\"\necho hi >&$fd\nexec {fd}>&-\ncat '" + dir + "/named'\necho again >&$fd 2>/dev/null || echo closed\n",
			want:   "fd=10\nhi\nclosed\n",
		},
		{
			name:   "each name gets the next free one",
			script: "exec {a}>/dev/null {b}>/dev/null\necho \"$a $b\"\n",
			want:   "10 11\n",
		},
		{
			name:   "a named descriptor reads",
			script: "printf 'l1\\nl2\\n' > '" + dir + "/in'\nexec {r}<'" + dir + "/in'\nread -r x <&$r\nread -r y <&$r\necho \"$x $y\"\n",
			want:   "l1 l2\n",
		},
		{
			name:   ">&word is both streams to a file",
			script: "x='" + dir + "/both'\n{ echo out; echo err >&2; } >&$x\ncat \"$x\"\n",
			want:   "out\nerr\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// A literal `>&file` is the same form, and it was a syntax error for the whole script.
func TestRedirect_literalFileAfterDuplication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "literal")
	script := "echo out >&" + filepath.ToSlash(path) + "\necho after\n"
	if stdout, _ := runScriptCapturing(script); stdout != "after\n" {
		t.Fatalf("stdout = %q, want the script to run", stdout)
	}
	if content, err := os.ReadFile(path); err != nil || string(content) != "out\n" {
		t.Fatalf("file = %q (err %v), want the output in it", content, err)
	}
}
