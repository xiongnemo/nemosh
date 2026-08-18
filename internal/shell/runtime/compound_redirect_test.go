package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
)

// A redirection after a compound's closer -- `done < file`, `fi > log`, `esac > out`.
//
// It reported `missing done for compound`, because the line `done < /dev/null` was not
// the word `done` and so nothing closed the loop. That rules out
// `while read -r line; do ...; done < file`, which is the way to read a file *without*
// a subshell -- the whole reason to write it that way rather than piping into the loop.
//
// A compound with a redirection is exactly a brace group holding that compound, and
// `{ ...; } < file` already worked, so the group is what it becomes. Reusing it rather
// than adding a second thing that carries redirections is what keeps the two from
// drifting apart.
func TestCompoundRedirect_readsAndWritesFiles(t *testing.T) {
	directory := t.TempDir()
	input := filepath.ToSlash(filepath.Join(directory, "in.txt"))
	if err := os.WriteFile(filepath.FromSlash(input), []byte("x\ny\n"), 0o644); err != nil {
		t.Fatalf("write the fixture: %v", err)
	}
	output := filepath.ToSlash(filepath.Join(directory, "out.txt"))

	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			// The idiom this exists for.
			name:   "while reading a file",
			script: "while read -r l; do printf '<%s>' \"$l\"; done < " + input + "\necho\n",
			want:   "<x><y>\n",
		},
		{
			name:   "while with a here-string",
			script: "while read -r l; do echo \"$l\"; done <<< \"hs\"\n", want: "hs\n",
		},
		{
			name:   "until reading a file",
			script: "until [ \"$l\" = y ]; do read -r l; done < " + input + "\necho \"$l\"\n", want: "y\n",
		},
		{
			// The variable survives, which is the difference from piping into the loop.
			name:   "the loop is not a subshell",
			script: "count=0\nwhile read -r l; do count=$((count+1)); done < " + input + "\necho \"$count\"\n",
			want:   "2\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}

	t.Run("writing from a for loop", func(t *testing.T) {
		// When
		status, stdout, stderr := runSetScript(t,
			"for i in a b; do echo $i; done > "+output+"\ncat "+output+"\n")

		// Then
		if status != 0 {
			t.Fatalf("status = %d, stderr = %q", status, stderr)
		}
		if stdout != "a\nb\n" {
			t.Fatalf("stdout = %q, want the loop's output to have gone to the file", stdout)
		}
	})

	t.Run("writing from an if", func(t *testing.T) {
		// When
		status, stdout, stderr := runSetScript(t,
			"if true; then echo t; fi > "+output+"\ncat "+output+"\n")

		// Then
		if status != 0 {
			t.Fatalf("status = %d, stderr = %q", status, stderr)
		}
		if stdout != "t\n" {
			t.Fatalf("stdout = %q, want the if's output in the file", stdout)
		}
	})

	t.Run("writing from a case", func(t *testing.T) {
		// When
		status, stdout, stderr := runSetScript(t,
			"case a in a) echo c ;; esac > "+output+"\ncat "+output+"\n")

		// Then
		if status != 0 {
			t.Fatalf("status = %d, stderr = %q", status, stderr)
		}
		if stdout != "c\n" {
			t.Fatalf("stdout = %q, want the case's output in the file", stdout)
		}
	})

	t.Run("appending", func(t *testing.T) {
		// When
		status, stdout, stderr := runSetScript(t,
			"echo first > "+output+"\nfor i in b; do echo $i; done >> "+output+"\ncat "+output+"\n")

		// Then
		if status != 0 {
			t.Fatalf("status = %d, stderr = %q", status, stderr)
		}
		if stdout != "first\nb\n" {
			t.Fatalf("stdout = %q, want the loop appended", stdout)
		}
	})
}

// The forms that were already right. The closer scan now accepts a suffix, and a word
// that merely begins with a closer must not be taken for one.
func TestCompoundRedirect_leavesOrdinaryCompoundsAlone(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a plain loop", script: "for i in a; do echo $i; done\n", want: "a\n"},
		{name: "a plain if", script: "if true; then echo t; fi\n", want: "t\n"},
		{name: "a plain case", script: "case a in a) echo c ;; esac\n", want: "c\n"},
		{name: "a plain while", script: "i=0\nwhile [ $i = 0 ]; do i=1; done\necho $i\n", want: "1\n"},
		{name: "a nested loop", script: "for i in a; do for j in b; do echo $i$j; done; done\n", want: "ab\n"},
		{name: "a group with a redirect still works", script: "{ echo g; } > /dev/null\necho after\n", want: "after\n"},
		{
			// `donefile` is a word, not a closer with a suffix.
			name: "a word beginning with a closer", script: "echo donefile\n", want: "donefile\n",
		},
		{name: "a variable named done", script: "done=1\necho $done\n", want: "1\n"},
		// Deliberately absent: an operator after a closer -- `esac && echo`,
		// `done | cat`. Those need the words after the compound built into pipelines or
		// an and-or list, which is more than a redirection needs. They did not work
		// before this change either: the suffix scan accepts only `<` and `>`, so that
		// path is exactly as it was.
		//
		// The brace group spelling works for a loop and for an if --
		// `{ for i in a; do echo $i; done; } && echo` prints both -- but not for a
		// case: a `case` inside `{ }` is itself refused, because the pattern's `)` is
		// taken for the group's closer. Measured, and recorded rather than half-fixed.
		{name: "a background loop", script: "for i in a; do echo $i; done &\nwait\n", want: "a\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}
