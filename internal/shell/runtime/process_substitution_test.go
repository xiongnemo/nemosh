package runtime_test

import (
	"os"
	"strings"
	"testing"
)

// `<(command)` was a syntax error, because `<` is a redirect operator and `(command)` a
// subshell. Every expectation below is measured from bash on this machine, except the two
// that are named as divergences.
func TestProcessSubstitution_readsACommandAsAFile(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "one operand", script: "cat <(echo hello)\n", want: "hello\n"},
		{
			name:   "three operands, in order",
			script: "cat <(echo a) <(echo b) <(echo c)\n", want: "a\nb\nc\n",
		},
		{
			// The point of the form: two commands compared without two files written
			// by hand. Exit 1 is diff's answer to differing input.
			name:   "diff, the form it exists for",
			script: "diff <(printf 'a\\nb\\n') <(printf 'a\\nc\\n')\necho status=$?\n",
			want:   "2c2\n< b\n---\n> c\nstatus=1\n",
		},
		{
			name:   "as a redirect target",
			script: "wc -l < <(printf 'x\\ny\\n')\n", want: "2\n",
		},
		{
			// The shape that makes the difference to a real script: a loop body that
			// keeps its variables, which `... | while read` cannot give.
			name:   "feeding a loop",
			script: "count=0\nwhile read line; do count=$((count+1)); done < <(printf '1\\n2\\n3\\n')\necho $count\n",
			want:   "3\n",
		},
		{name: "a compound inside it", script: "cat <(echo a; echo b)\n", want: "a\nb\n"},
		{name: "a pipeline inside it", script: "cat <(echo ab | wc -c)\n", want: "3\n"},
		{name: "nested", script: "cat <(cat <(echo deep))\n", want: "deep\n"},
		{
			// The command runs in a snapshot, so it reads the shell's variables and
			// cannot write them.
			name:   "a snapshot, not the shell",
			script: "y=0\ncat <(y=9; echo inner=$y)\necho outer=$y\n", want: "inner=9\nouter=0\n",
		},
		{
			// The consumer's status is the command's status; the substitution's is
			// discarded.
			name:   "the substitution's status is not the command's",
			script: "cat <(false)\necho status=$?\n", want: "status=0\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q; stderr = %q", stdout, test.want, stderr)
			}
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
		})
	}
}

// Inside a command substitution the parentheses have to balance for two scans at once.
// Before this the body was cut off at the substitution's first `)` and the leftovers were
// reported as `syntax error: unexpected )`.
func TestProcessSubstitution_insideACommandSubstitution(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "unquoted", script: "echo $(cat <(echo n))\n", want: "n\n"},
		{name: "quoted", script: "echo \"$(cat <(echo q))\"\n", want: "q\n"},
		{name: "assigned", script: "f=$(cat <(echo v))\necho f=$f\n", want: "f=v\n"},
		// Two more the same scan was getting wrong, found while fixing it: a bare
		// subshell and a case both put a `)` where the scan counted one of its own.
		{name: "a subshell, which had the same defect", script: "echo $( (echo bare) )\n", want: "bare\n"},
		{
			name:   "a case, which had it too",
			script: "echo $(case x in x) echo hit;; esac)\n", want: "hit\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.want || status != 0 {
				t.Fatalf("stdout = %q status = %d, want %q and 0; stderr = %q",
					stdout, status, test.want, stderr)
			}
		})
	}
}

// The output form is refused rather than approximated. Writing into a file after the
// command that would read it has already finished is not what `>(command)` means, and a
// script that got a path back would be told the wrong thing quietly.
func TestProcessSubstitution_refusesTheOutputForm(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "echo hi >(cat)\n")

	// Then
	if status == 0 {
		t.Fatalf("status = 0, want a failure; stdout = %q", stdout)
	}
	for _, want := range []string{">(cat)", "<(...)"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want it to name %q", stderr, want)
		}
	}
}

// A path rather than `/dev/fd/63`, and a temporary file that is removed when the command
// that reads it has finished. Both are divergences from bash worth pinning: Windows has no
// `/dev/fd`, so the file is real, and a real file has to be cleaned up. See
// process_substitution.go for why a temporary file rather than a named pipe.
func TestProcessSubstitution_substitutesARealPathAndRemovesIt(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "echo <(true)\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	path := strings.TrimSpace(stdout)
	if !strings.Contains(path, "nemosh-procsub-") {
		t.Fatalf("stdout = %q, want a temporary file path", stdout)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(%q) = %v, want the file to be gone once the command has run", path, err)
	}
}

// The path names a regular file, which is the whole of what the temporary-file decision
// buys, so the tests that ask about it have to agree. `-f` answers false in bash, where the
// path names a pipe; pinned here so it is a decision rather than an accident.
func TestProcessSubstitution_thePathIsARegularFile(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t,
		"[[ -f <(echo x) ]] && echo isfile\ntest -r <(echo y) && echo readable\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if stdout != "isfile\nreadable\n" {
		t.Fatalf("stdout = %q, want both tests to pass -- bash says false to the first", stdout)
	}
}

// A substitution in a loop must not leave one file per iteration behind.
func TestProcessSubstitution_removesOneFilePerIteration(t *testing.T) {
	before := countSubstitutionFiles(t)

	// When
	status, stdout, stderr := runSetScript(t,
		"for i in 1 2 3 4 5; do cat <(echo $i); done\n")

	// Then
	if status != 0 || stdout != "1\n2\n3\n4\n5\n" {
		t.Fatalf("status = %d stdout = %q, stderr = %q", status, stdout, stderr)
	}
	if after := countSubstitutionFiles(t); after != before {
		t.Fatalf("%d temporary files before and %d after, want them all removed", before, after)
	}
}

func countSubstitutionFiles(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatalf("read %s: %v", os.TempDir(), err)
	}
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "nemosh-procsub-") {
			count++
		}
	}
	return count
}
