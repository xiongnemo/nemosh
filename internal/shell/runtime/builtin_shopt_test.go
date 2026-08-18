package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `shopt` was not a builtin, so `shopt -s globstar` failed with `shopt: not found`
// and the three glob options it is nearly always used for could not be turned on.
//
// The tree is built on disk because these are questions about the filesystem, and the
// expectations were measured against bash over the same tree.
func globTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{"a", "a/b", "a/b/c"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	for _, file := range []string{"top.go", "a/one.go", "a/b/two.go", "a/b/c/three.go", ".hidden"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(file)), nil, 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}
	return filepath.ToSlash(root)
}

func TestShopt_globstarCrossesDirectories(t *testing.T) {
	root := globTree(t)

	t.Run("on", func(t *testing.T) {
		// When
		status, stdout, stderr := runSetScript(t,
			"cd "+root+"\nshopt -s globstar\nfor f in **/*.go; do printf '%s ' \"$f\"; done\necho\n")

		// Then -- measured from bash over the same tree.
		if status != 0 {
			t.Fatalf("status = %d, stderr = %q", status, stderr)
		}
		if want := "a/b/c/three.go a/b/two.go a/one.go top.go \n"; stdout != want {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	})

	t.Run("off, where ** is an ordinary star", func(t *testing.T) {
		// When
		status, stdout, stderr := runSetScript(t,
			"cd "+root+"\nfor f in **/*.go; do printf '%s ' \"$f\"; done\necho\n")

		// Then
		if status != 0 {
			t.Fatalf("status = %d, stderr = %q", status, stderr)
		}
		if want := "a/one.go \n"; stdout != want {
			t.Fatalf("stdout = %q, want %q -- without globstar, ** is one segment", stdout, want)
		}
	})
}

func TestShopt_nullglobDropsAPatternThatMatchesNothing(t *testing.T) {
	root := globTree(t)
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			// POSIX: the pattern stays as written, so the loop runs once over it.
			name: "off", script: "cd " + root + "\nfor f in *.none; do echo \"[$f]\"; done\n", want: "[*.none]\n",
		},
		{
			name: "on", script: "cd " + root + "\nshopt -s nullglob\nfor f in *.none; do echo \"[$f]\"; done\necho end\n",
			want: "end\n",
		},
		{
			// A pattern that does match is unaffected either way.
			name: "on, with a match", script: "cd " + root + "\nshopt -s nullglob\necho *.go\n", want: "top.go\n",
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
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

func TestShopt_dotglobIncludesLeadingDots(t *testing.T) {
	root := globTree(t)

	// When
	_, without, _ := runSetScript(t, "cd "+root+"\necho *\n")
	_, with, _ := runSetScript(t, "cd "+root+"\nshopt -s dotglob\necho *\n")

	// Then
	if strings.Contains(without, ".hidden") {
		t.Fatalf("without dotglob the listing holds .hidden: %q", without)
	}
	if !strings.Contains(with, ".hidden") {
		t.Fatalf("with dotglob the listing does not hold .hidden: %q", with)
	}
}

func TestShopt_reportsAndSetsState(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		contains string
		status   int
	}{
		{name: "the listing", script: "shopt\n", contains: "globstar", status: 0},
		{name: "after setting", script: "shopt -s globstar\nshopt\n", contains: "globstar    \ton", status: 0},
		{name: "after unsetting", script: "shopt -s globstar\nshopt -u globstar\nshopt\n", contains: "globstar    \toff", status: 0},
		{name: "-q on an option that is off", script: "shopt -q globstar\n", contains: "", status: 1},
		{name: "-q on an option that is on", script: "shopt -s nullglob\nshopt -q nullglob\n", contains: "", status: 0},
		{
			// Refused rather than accepted: accepting it would leave a script
			// believing `@(a|b)` works.
			name: "an option this build does not have", script: "shopt -s extglob\n",
			contains: "not an option this build has", status: 1,
		},
		{
			name: "the set -o options are not shopt's", script: "shopt -o errexit\n",
			contains: "reached with `set`", status: 2,
		},
		{name: "-s and -u together", script: "shopt -s -u globstar\n", contains: "cannot both be given", status: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != test.status {
				t.Fatalf("status = %d, want %d; stderr = %q", status, test.status, stderr)
			}
			if test.contains != "" && !strings.Contains(stdout+stderr, test.contains) {
				t.Fatalf("output = %q / %q, want it to contain %q", stdout, stderr, test.contains)
			}
		})
	}
}

// `set -o nocaseglob` and `shopt -s nocaseglob` are the same switch, so one cannot be
// on while the other reads off.
func TestShopt_sharesNocaseglobWithSet(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "set -o nocaseglob\nshopt -q nocaseglob\necho \"q=$?\"\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if stdout != "q=0\n" {
		t.Fatalf("stdout = %q, want q=0 -- the two spellings are one option", stdout)
	}
}
