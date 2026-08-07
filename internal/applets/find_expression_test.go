package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// findFixture builds the tree every case below walks:
//
//	.            directory
//	a.txt        file
//	notes.md     file
//	sub/         directory
//	sub/b.txt    file
func findFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"a.txt":         "a",
		"notes.md":      "n",
		"sub/b.txt":     "b",
		"sub/keep.note": "k",
	} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(path)), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func runFind(t *testing.T, dir string, args ...string) (string, string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("find")
	if !ok {
		t.Fatal("find is not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: dir})
	err := applet.Run(ctx, args, &bytes.Buffer{}, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func TestFind_appliesTheExpression(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "no expression walks everything",
			args: []string{"."},
			want: []string{".", "./a.txt", "./notes.md", "./sub", "./sub/b.txt", "./sub/keep.note"},
		},
		{
			name: "name matches the basename, not the path",
			args: []string{".", "-name", "b.txt"},
			want: []string{"./sub/b.txt"},
		},
		{
			name: "name takes a pattern",
			args: []string{".", "-name", "*.txt"},
			want: []string{"./a.txt", "./sub/b.txt"},
		},
		{
			name: "a pattern matching nothing prints nothing",
			args: []string{".", "-name", "*.nosuch"},
			want: nil,
		},
		{
			name: "type f selects regular files",
			args: []string{".", "-type", "f"},
			want: []string{"./a.txt", "./notes.md", "./sub/b.txt", "./sub/keep.note"},
		},
		{
			name: "type d selects directories, including the root",
			args: []string{".", "-type", "d"},
			want: []string{".", "./sub"},
		},
		{
			name: "predicates combine with an implicit and",
			args: []string{".", "-type", "f", "-name", "*.txt"},
			want: []string{"./a.txt", "./sub/b.txt"},
		},
		{
			name: "an explicit print is the same as the implied one",
			args: []string{".", "-name", "a.txt", "-print"},
			want: []string{"./a.txt"},
		},
		{
			name: "print alone still walks everything",
			args: []string{".", "-print"},
			want: []string{".", "./a.txt", "./notes.md", "./sub", "./sub/b.txt", "./sub/keep.note"},
		},
		{
			name: "a bracket pattern",
			args: []string{".", "-name", "[ab].txt"},
			want: []string{"./a.txt", "./sub/b.txt"},
		},
		{
			name: "the path defaults to the working directory",
			args: []string{"-name", "notes.md"},
			want: []string{"./notes.md"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			dir := findFixture(t)

			// When
			stdout, stderr, err := runFind(t, dir, test.args...)

			// Then
			if err != nil {
				t.Fatalf("find %v: %v (stderr %q)", test.args, err, stderr)
			}
			var got []string
			if trimmed := strings.TrimSuffix(stdout, "\n"); trimmed != "" {
				got = strings.Split(trimmed, "\n")
			}
			if strings.Join(got, "|") != strings.Join(test.want, "|") {
				t.Fatalf("find %v\n  got  %v\n  want %v", test.args, got, test.want)
			}
		})
	}
}

// The whole point of this change: an expression find cannot evaluate is refused
// *before* the walk, so no wrong path is ever written. `find . -name '*.tmp' |
// xargs rm` used to receive every file in the tree.
func TestFind_refusesAnUnsupportedExpression_beforeWritingAnything(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     []string
		wantWord string
	}{
		{name: "an unknown predicate", args: []string{".", "-nosuchpred"}, wantWord: "-nosuchpred"},
		{name: "a predicate busybox has but this does not", args: []string{".", "-mtime", "1"}, wantWord: "-mtime"},
		{name: "name without its pattern", args: []string{".", "-name"}, wantWord: "-name"},
		{name: "type without its letter", args: []string{".", "-type"}, wantWord: "-type"},
		{name: "a type letter that is not supported", args: []string{".", "-type", "s"}, wantWord: "type"},
		{name: "a type letter that is not a letter", args: []string{".", "-type", "ff"}, wantWord: "type"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			dir := findFixture(t)

			// When
			stdout, stderr, err := runFind(t, dir, test.args...)

			// Then
			if err == nil {
				t.Fatalf("find %v succeeded, want a refusal", test.args)
			}
			if stdout != "" {
				t.Fatalf("find %v wrote %q to stdout before refusing", test.args, stdout)
			}
			message := stderr + err.Error()
			if !strings.Contains(message, test.wantWord) {
				t.Fatalf("find %v reported %q, want it to name %q", test.args, message, test.wantWord)
			}
			// The old failure said "No such file or directory", which sends the
			// reader looking for a file that was never meant to be one.
			if strings.Contains(message, "No such file") {
				t.Fatalf("find %v still reports an expression as a missing file: %q", test.args, message)
			}
		})
	}
}

func TestFind_walksSeveralPaths(t *testing.T) {
	// Given
	dir := findFixture(t)

	// When
	stdout, stderr, err := runFind(t, dir, "sub", "notes.md", "-name", "*.txt")

	// Then
	if err != nil {
		t.Fatalf("find: %v (stderr %q)", err, stderr)
	}
	if got := stdout; got != "sub/b.txt\n" {
		t.Fatalf("stdout = %q, want %q", got, "sub/b.txt\n")
	}
}
