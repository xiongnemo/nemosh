package runtime_test

import (
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"testing"
)

// linkedDirectory makes real/ and link/, a link to it: a junction on Windows, which needs no
// privilege where a symbolic link does, and a symbolic link elsewhere.
func linkedDirectory(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	real, link := filepath.Join(root, "real"), filepath.Join(root, "link")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if goruntime.GOOS == "windows" {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, real).CombinedOutput(); err != nil {
			t.Skipf("no junction can be made here: %v %s", err, out)
		}
		return filepath.ToSlash(root)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("no symbolic link can be made here: %v", err)
	}
	return filepath.ToSlash(root)
}

// cd and pwd take -L and -P, as busybox's do: -P follows the link, a junction as much as a
// symbolic link, and the last one given wins. They were taken for directory names.
func TestDirectory_logicalAndPhysical(t *testing.T) {
	root := linkedDirectory(t)
	where := "where() { case $1 in */real) echo real;; */link) echo link;; *) echo \"? $1\";; esac; }\n"
	tests := []struct {
		name, script, stdout string
		status               int
	}{
		{name: "logical by default", script: "cd link\nwhere \"$(pwd)\"\nwhere \"$(pwd -P)\"\nwhere \"$(pwd -L)\"\n", stdout: "link\nreal\nlink\n"},
		{name: "cd -P", script: "cd -P link\nwhere \"$(pwd)\"\nwhere \"$PWD\"\n", stdout: "real\nreal\n"},
		{name: "the last one wins", script: "cd -LP link\nwhere \"$(pwd)\"\ncd ..\ncd -PL link\nwhere \"$(pwd)\"\n", stdout: "real\nlink\n"},
		{name: "-- ends them", script: "cd -- link\nwhere \"$(pwd)\"\n", stdout: "link\n"},
		{
			// pwd's too: after `cd -L link`, $PWD is the link and `pwd` the directory, in bash.
			name:   "set -P makes -P the default",
			script: "set -P\ncd link\nwhere \"$(pwd)\"\ncd ..\ncd -L link\nwhere \"$PWD\"\nwhere \"$(pwd)\"\n",
			stdout: "real\nlink\nreal\n",
		},
		{name: "an option neither has", script: "cd -x link 2>&1\necho \"st=$?\"\npwd -x 2>&1\necho \"st=$?\"\n", stdout: "cd: illegal option -x\nst=2\npwd: illegal option -x\nst=2\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, "cd '"+root+"'\n"+where+test.script)

			// Then
			if stdout != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d; stderr = %q", stdout, status, test.stdout, test.status, stderr)
			}
		})
	}
}
