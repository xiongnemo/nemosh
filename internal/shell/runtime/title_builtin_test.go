package runtime

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// fakeTitle is a console's title, held, or a console that cannot be asked.
type fakeTitle struct {
	text string
	err  error
}

func (f *fakeTitle) title() (string, error) { return f.text, f.err }

func (f *fakeTitle) setTitle(text string) error {
	if f.err != nil {
		return f.err
	}
	f.text = text
	return nil
}

// title is busybox-w32's (shell/ash.c:16063): an operand is the console's title -- the first,
// whatever it looks like, `-x` too, for busybox reads no options -- and none prints the title.
func TestTitle_setsTheTitleToItsFirstOperandAndPrintsItWithNone(t *testing.T) {
	for _, test := range []struct {
		args          []string
		before, after string
		stdout        string
	}{
		{args: []string{"build"}, before: "old", after: "build"},
		{args: []string{"-x"}, before: "old", after: "-x"},
		{args: []string{"a", "b"}, before: "old", after: "a"},
		{args: nil, before: "where", after: "where", stdout: "where\n"},
		{args: []string{""}, before: "old", after: ""},
	} {
		console := &fakeTitle{text: test.before}
		var stdout, stderr bytes.Buffer

		// When
		status := runTitle(test.args, console, &stdout, &stderr, "nemosh: ")

		// Then
		if status != 0 || console.text != test.after || stdout.String() != test.stdout || stderr.Len() != 0 {
			t.Errorf("title %q: status %d, title %q, printed %q, %q; want 0, %q, %q", test.args, status, console.text,
				stdout.String(), stderr.String(), test.after, test.stdout)
		}
	}
}

// busybox says nothing and answers 0 when the console cannot be asked; this says why and
// answers 1, as a capability that is absent must.
func TestTitle_saysWhyAConsoleCannotBeAsked(t *testing.T) {
	for _, args := range [][]string{{"x"}, nil} {
		console := &fakeTitle{err: errors.New("no console")}
		var stdout, stderr bytes.Buffer

		// When
		status := runTitle(args, console, &stdout, &stderr, "nemosh: ")

		// Then
		if status != 1 || stderr.String() != "nemosh: title: no console\n" || stdout.Len() != 0 {
			t.Errorf("title %q with no console: status %d, %q, %q", args, status, stdout.String(), stderr.String())
		}
	}
}

// It is a builtin, which type, help and --help know.
func TestTitle_isABuiltin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})

	// When
	rt.RunScript(t.Context(), "type title\ntitle --help\n")

	// Then
	if !strings.HasPrefix(stdout.String(), "title is a shell builtin\n") || !strings.Contains(stdout.String(), "Usage: title") {
		t.Fatalf("printed %q, %q", stdout.String(), stderr.String())
	}
}
