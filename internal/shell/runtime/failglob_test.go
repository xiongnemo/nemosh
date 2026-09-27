package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
)

// `shopt -s failglob` makes a pattern that matches nothing an error, where it otherwise
// stays as written. As in bash, the error abandons the whole command the script was running
// -- a function it called, a loop it was in -- with status 1, and the script goes on with the
// next one; under `set -e` it ends there. A redirection's pattern fails only its command.
// busybox has no shopt.
func TestShopt_failglob(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "a.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cd := "cd '" + filepath.ToSlash(directory) + "'\n"
	tests := []struct {
		name, script, stdout string
		status               int
	}{
		{name: "a command", script: "echo *.ZZ\nshopt -s failglob\necho *.ZZ\necho status=$?\n", stdout: "*.ZZ\nstatus=1\n"},
		{
			name:   "a loop's words",
			script: "for x in *.ZZ; do echo $x; done\nshopt -s failglob\nfor x in *.ZZ; do echo $x; done\necho status=$?\n",
			stdout: "*.ZZ\nstatus=1\n",
		},
		{name: "an array literal", script: "shopt -s failglob\nmyarr=(*.ZZ)\necho \"status=$? ${#myarr[@]}\"\n", stdout: "status=1 0\n"},
		{
			name:   "the function it was in is abandoned",
			script: "shopt -s failglob\nf() {\n  echo *.ZZ\n  echo in-f\n}\nf\necho \"after st=$?\"\n",
			stdout: "after st=1\n",
		},
		{
			name:   "and the loop",
			script: "shopt -s failglob\nfor i in 1 2; do\n  echo \"loop $i\"\n  echo *.ZZ\ndone\necho \"st=$?\"\n",
			stdout: "loop 1\nst=1\n",
		},
		{name: "a subshell ends", script: "shopt -s failglob\n(echo *.ZZ; echo in-sub)\necho \"st=$?\"\n", stdout: "st=1\n"},
		{name: "set -e ends the script", script: "set -e\nshopt -s failglob\necho *.ZZ\necho after\n", stdout: "", status: 1},
		{name: "a redirection", script: "shopt -s failglob\necho hi > zz-*-xx; echo \"st=$?\"\n", stdout: "st=1\n"},
		{name: "before nullglob", script: "shopt -s failglob nullglob\necho x *.ZZ y\necho \"st=$?\"\n", stdout: "st=1\n"},
		{name: "a match is a match", script: "shopt -s failglob\necho *.txt\n", stdout: "a.txt\n"},
		{
			name:   "patterns that are not paths",
			script: "shopt -s failglob\ncase x in *.ZZ) ;; esac\n[[ a == *.ZZ ]]\necho ok\n",
			stdout: "ok\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, cd+test.script)

			// Then
			if stdout != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d; stderr = %q", stdout, status, test.stdout, test.status, stderr)
			}
		})
	}
}
