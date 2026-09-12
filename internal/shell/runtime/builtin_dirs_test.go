package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The directory stack, every expectation measured against bash on the same input.
//
// The measuring mattered more here than usual, because two of these are counter-intuitive
// and I had both wrong first time:
//
//   - `-N` counts from the *far end*, so `-0` is the oldest entry and not the newest.
//     Three call sites needed the fold and only one had it, so `pushd +N` and `dirs -0`
//     disagreed about what the same operand meant.
//   - `pushd +N` **rotates** the stack; it does not move to entry N and discard the rest.
//     `popd +N` removes. The two look alike and do different things.

// dirsRoot makes a root with two subdirectories and answers a script prefix that starts
// there, plus a replacer that hides the temporary path from the assertions.
func dirsRoot(t *testing.T) (prefix string, hide func(string) string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	slashed := filepath.ToSlash(root)
	// The shell answers in its own spelling -- `/c/Users/...` where the host says
	// `C:/Users/...` -- so all three forms are hidden. Without the first of them these
	// assertions compared a nemosh path against a Windows one and every case failed on
	// the drive letter rather than on anything being tested.
	spellings := []string{driveLetterToNemosh(slashed), slashed, root}
	return "cd " + slashed + "\n", func(text string) string {
		for _, spelling := range spellings {
			if spelling != "" {
				text = strings.ReplaceAll(text, spelling, "R")
			}
		}
		return text
	}
}

// driveLetterToNemosh rewrites `C:/x` as `/c/x`, which is how this shell prints a path.
func driveLetterToNemosh(slashed string) string {
	if len(slashed) < 2 || slashed[1] != ':' {
		return ""
	}
	return "/" + strings.ToLower(slashed[:1]) + slashed[2:]
}

func TestDirs_stack(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		// Position zero is the current directory and is not stored, so a plain `cd`
		// moves it. Anything that kept its own copy would show the old one here.
		{name: "just the current directory", script: `dirs`, want: "R\n"},
		{name: "cd moves the top", script: "cd a\ndirs", want: "R/a\n"},

		{name: "pushd stacks", script: "pushd a >/dev/null\ndirs", want: "R/a R\n"},
		{name: "twice", script: "pushd a >/dev/null\npushd ../b >/dev/null\ndirs", want: "R/b R/a R\n"},
		{name: "pushd prints the stack", script: "pushd a", want: "R/a R\n"},
		{name: "popd returns", script: "pushd a >/dev/null\npopd >/dev/null\npwd", want: "R\n"},

		// No operand exchanges the top two, which is the there-and-back people use.
		{name: "bare pushd swaps", script: "pushd a >/dev/null\npushd\npwd", want: "R R/a\nR\n"},

		// `+N` counts from the current directory, `-N` from the far end.
		{name: "dirs +0", script: "pushd a >/dev/null\npushd ../b >/dev/null\ndirs +0", want: "R/b\n"},
		{name: "dirs +2", script: "pushd a >/dev/null\npushd ../b >/dev/null\ndirs +2", want: "R\n"},
		{name: "dirs -0 is the oldest", script: "pushd a >/dev/null\npushd ../b >/dev/null\ndirs -0", want: "R\n"},
		{name: "dirs -2 is the newest", script: "pushd a >/dev/null\npushd ../b >/dev/null\ndirs -2", want: "R/b\n"},

		// pushd rotates; popd removes. The pair that is easy to conflate.
		{
			name:   "pushd +1 rotates",
			script: "pushd a >/dev/null\npushd ../b >/dev/null\npushd +1 >/dev/null\ndirs",
			want:   "R/a R R/b\n",
		},
		{
			name:   "popd +1 removes",
			script: "pushd a >/dev/null\npushd ../b >/dev/null\npopd +1 >/dev/null\ndirs",
			want:   "R/b R\n",
		},
		{
			// Removing something that is not the top leaves the shell where it is.
			name:   "popd +1 does not move",
			script: "pushd a >/dev/null\npushd ../b >/dev/null\npopd +1 >/dev/null\npwd",
			want:   "R/b\n",
		},

		// The listing forms.
		{name: "dirs -p", script: "pushd a >/dev/null\ndirs -p", want: "R/a\nR\n"},
		{name: "dirs -v", script: "pushd a >/dev/null\ndirs -v", want: " 0  R/a\n 1  R\n"},
		{name: "dirs -c empties it", script: "pushd a >/dev/null\ndirs -c\ndirs", want: "R/a\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			prefix, hide := dirsRoot(t)
			status, stdout, stderr := runSetScript(t, prefix+test.script+"\n")
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, hide(stderr))
			}
			if got := hide(stdout); got != test.want {
				t.Fatalf("%s\n  got  %q\n  want %q", test.script, got, test.want)
			}
		})
	}
}

// The refusals, which have to be loud: a stack operation that silently does nothing
// leaves the shell somewhere the script did not intend.
func TestDirs_refusals(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		says   string
	}{
		{name: "popd on an empty stack", script: "popd", says: "directory stack empty"},
		{name: "bare pushd with nothing under it", script: "pushd", says: "no other directory"},
		{name: "index past the end", script: "pushd a >/dev/null\ndirs +9", says: "out of range"},
		{name: "popd index past the end", script: "pushd a >/dev/null\npopd +9", says: "out of range"},
		{name: "an option that is not one", script: "dirs -z", says: "invalid option"},
		{name: "two directories", script: "pushd a b", says: "too many arguments"},
		// And the one that blamed the wrong builtin: pushd moves by calling cd's body,
		// and before that body took a name to report as, `pushd nosuchdir` said `cd:`.
		{name: "a directory that is not there", script: "pushd nosuchdir", says: "pushd: nosuchdir: No such file or directory"},
	} {
		t.Run(test.name, func(t *testing.T) {
			prefix, hide := dirsRoot(t)
			status, _, stderr := runSetScript(t, prefix+test.script+"\n")
			if status == 0 {
				t.Fatalf("%q succeeded", test.script)
			}
			if !strings.Contains(hide(stderr), test.says) {
				t.Fatalf("%q said %q, which does not contain %q", test.script, hide(stderr), test.says)
			}
		})
	}
}

// A subshell gets a copy of the stack, so what it pushes does not escape -- the rule
// arrays follow. $SECONDS and history are the deliberate exceptions and this is not one.
func TestDirs_subshellKeepsItsPushesToItself(t *testing.T) {
	prefix, hide := dirsRoot(t)
	status, stdout, stderr := runSetScript(t,
		prefix+"pushd a >/dev/null\n(pushd ../b >/dev/null)\ndirs\n")
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, hide(stderr))
	}
	if got, want := hide(stdout), "R/a R\n"; got != want {
		t.Fatalf("the subshell's pushd escaped:\n  got  %q\n  want %q", got, want)
	}
}

// The home directory is shown as `~`, which is what makes a stack of deep paths readable
// on one line -- and the abbreviation must not fire on a path that merely starts with the
// same letters.
func TestDirs_abbreviatesHome(t *testing.T) {
	prefix, _ := dirsRoot(t)
	status, stdout, stderr := runSetScript(t, prefix+"HOME=/tmp\ncd /tmp\ndirs\n")
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if got := strings.TrimSpace(stdout); got != "~" {
		t.Fatalf("the home directory is shown as %q, want ~", got)
	}
	// -l spells it out again, which is what that option is for.
	status, stdout, _ = runSetScript(t, prefix+"HOME=/tmp\ncd /tmp\ndirs -l\n")
	if status != 0 || strings.TrimSpace(stdout) == "~" {
		t.Fatalf("dirs -l abbreviated anyway: %q", stdout)
	}
}
